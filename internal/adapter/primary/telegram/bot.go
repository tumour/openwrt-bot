// Package telegram — driving adapter, инкапсулирует работу с Telegram (telebot.v3).
// Внешние слои (cmd/bot/main.go) знают только NewBot и Run — никаких ссылок на tele.
package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/tumour/openwrt-bot/internal/adapter/primary/telegram/middleware"
	tele "gopkg.in/telebot.v3"
)

// Config — настройки бота. Tokенов мы тут не валидируем (это работа config layer).
type Config struct {
	Token          string
	AllowedUserIDs []int64 // whitelist Telegram user ID (отправители, не чаты)
}

// Bot — обёртка над telebot.Bot с lifecycle-методом Run, удобным для main.
type Bot struct {
	bot    *tele.Bot
	logger *slog.Logger

	// menu — предвычисленные пункты меню: SetCommands сетевой, поэтому уезжает
	// из конструктора в post-connect фазу Run.
	menu []tele.Command
	// connectBackoff — ритм дозвона до Telegram; поле, а не локальная переменная,
	// чтобы тесты инжектили малые значения.
	connectBackoff backoff

	// runCtx — базовый ctx из Run. Пишется один раз ДО старта поллера
	// (go-statement даёт happens-before), читается track-middleware.
	runCtx context.Context
	// wg учитывает in-flight handlers: telebot запускает каждый в горутине
	// и в Stop() их НЕ дожидается — ждём сами в Run, иначе процесс выходит
	// посреди работающего handler'а.
	wg sync.WaitGroup
}

// NewBot собирает telebot БЕЗ похода в сеть (Offline: getMe уезжает в
// connect-фазу Run) — процесс поднимается и при лежащем VPN. Вся проводка
// (middleware, routes) остаётся здесь: ошибки сборки (nil handler и т.п.)
// всплывают на старте, а не когда появится сеть. Handlers инжектируются
// снаружи — это позволяет тестировать бота с фейк-handler'ами.
func NewBot(cfg Config, logger *slog.Logger, h Handlers) (*Bot, error) {
	tb, err := tele.NewBot(tele.Settings{
		Token:   cfg.Token,
		Offline: true,
		Poller:  newResilientPoller(logger),
	})
	if err != nil {
		return nil, fmt.Errorf("init telebot: %w", err)
	}

	b := &Bot{
		bot:            tb,
		logger:         logger,
		menu:           menuCommands(h),
		connectBackoff: newBackoff(),
	}

	// Порядок middleware важен: сначала Auth (чтобы не логировать спам от чужих
	// и не считать его in-flight), затем track (учёт + базовый ctx), потом Log.
	tb.Use(middleware.Auth(cfg.AllowedUserIDs, logger))
	tb.Use(b.track)
	tb.Use(middleware.Log(logger))

	registerRoutes(tb, h)

	return b, nil
}

// track сопровождает каждый авторизованный апдейт: кладёт базовый ctx приложения
// (handlers строят от него таймауты → shutdown отменяет их exec'и) и регистрирует
// handler-горутину в WaitGroup, чтобы Run дождался её на выходе.
func (b *Bot) track(next tele.HandlerFunc) tele.HandlerFunc {
	return func(c tele.Context) error {
		b.wg.Add(1)
		defer b.wg.Done()
		middleware.PutBaseContext(c, b.runCtx)
		return next(c)
	}
}

// Run блокирующе работает до отмены ctx, тремя фазами: дозвон до Telegram
// (retry с backoff — «живучий старт», процесс не умирает ни при лежащем VPN,
// ни при битом токене), установка меню, поллинг. telebot.Start блокирующий,
// поэтому запускаем его в отдельной горутине и сигнализируем через done.
// На отмене ctx зовём Stop (корректно завершает long-poll-цикл) и дожидаемся
// in-flight handlers.
func (b *Bot) Run(ctx context.Context) error {
	b.runCtx = ctx // до любых горутин — happens-before для track

	// Фаза 1: дозвон. КРИТИЧНО: при отмене ctx здесь выходим БЕЗ b.bot.Stop() —
	// Start() ещё не вызывался, а Stop() без запущенного Start() виснет навсегда
	// (send в небуферизованный b.stop, telebot bot.go).
	if !b.connect(ctx) {
		b.logger.Info("shutting down before telegram connected")
		return nil
	}

	// Фаза 2: нативное меню команд (кнопка ≡ + автодополнение по «/»).
	// Некритично: без меню бот работает, поэтому Warn, а не смерть.
	if err := b.bot.SetCommands(b.menu); err != nil {
		b.logger.Warn("не удалось установить меню команд Telegram", "err", err)
	}

	// Фаза 3: поллинг.
	done := make(chan struct{})
	go func() {
		defer close(done)
		b.bot.Start()
	}()

	select {
	case <-ctx.Done():
		b.logger.Info("shutting down telegram bot")
		b.bot.Stop() // останавливает поллер; in-flight handlers НЕ ждёт
		<-done
		// ctx уже отменён → handler'ы, строящие таймауты от BaseContext,
		// прерываются сразу. Потолок — на случай зависшего HTTP к Telegram.
		b.waitHandlers(5 * time.Second)
		return nil
	case <-done:
		// telebot.Start вернулся сам по себе. С resilientPoller это недостижимо
		// by-design (поллер не сдаётся), но если случится (паника в кишках
		// telebot) — один honest-выход, procd перезапустит; это не crash-loop.
		return fmt.Errorf("telegram bot stopped unexpectedly")
	}
}

// connect дозванивается до Telegram с backoff: true — подключились (bot.Me
// заполнен), false — ctx отменён. Не сдаётся никогда: даже битый токен
// (401/404) даёт Error с подсказкой и retry — процесс должен жить ради
// HTTP API и procd (замена токена в env + bot restart чинят без crash-loop).
func (b *Bot) connect(ctx context.Context) bool {
	var lastAuth, warned bool
	for {
		user, err := b.probeGetMe(ctx)
		if err == nil {
			// До go Start() — happens-before; telebot читает Me только при
			// обработке апдейтов, а они текут после Start.
			b.bot.Me = user
			b.logger.Info("telegram connected", "username", user.Username)
			return true
		}
		if ctx.Err() != nil {
			return false
		}

		delay := b.connectBackoff.next()
		// State-transition логи (как в поллере): Warn/Error на смене состояния,
		// Debug на повторах — ночь без VPN не вымывает кольцо logread.
		auth := errors.Is(err, tele.ErrUnauthorized) || errors.Is(err, tele.ErrNotFound)
		switch {
		case auth && !lastAuth:
			b.logger.Error("telegram отверг токен — проверь BOT_TOKEN в /etc/openwrt-bot/env", "err", err)
		case !auth && (!warned || lastAuth):
			b.logger.Warn("telegram недоступен, буду пытаться в фоне", "err", err, "retry_in", delay)
		default:
			b.logger.Debug("telegram всё ещё недоступен", "err", err, "retry_in", delay)
		}
		lastAuth, warned = auth, true

		select {
		case <-ctx.Done():
			return false
		case <-time.After(delay):
		}
	}
}

// probeGetMe — getMe через публичный Raw, отменяемый ctx'ом. До Start() у
// telebot нет stopClient, т.е. сам Raw неотменяем (только таймаут клиента,
// 1 мин) — гоняем его в горутине и бросаем при отмене: горутина доживёт в
// фоне не дольше таймаута, а процесс выходит сразу. Формат ответа — паритет
// с telebot getMe (api.go): struct{ Result *User }.
func (b *Bot) probeGetMe(ctx context.Context) (*tele.User, error) {
	type result struct {
		user *tele.User
		err  error
	}
	ch := make(chan result, 1) // буфер: горутина не зависнет после отмены
	go func() {
		data, err := b.bot.Raw("getMe", nil)
		if err != nil {
			ch <- result{nil, err}
			return
		}
		var resp struct {
			Result *tele.User
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			ch <- result{nil, err}
			return
		}
		ch <- result{resp.Result, nil}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		return r.user, r.err
	}
}

// waitHandlers ждёт in-flight handlers, но не дольше d: один зависший вызов
// Telegram API не должен бесконечно держать рестарт сервиса.
func (b *Bot) waitHandlers(d time.Duration) {
	finished := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(finished)
	}()
	select {
	case <-finished:
	case <-time.After(d):
		b.logger.Warn("не все handlers завершились до таймаута, выходим")
	}
}

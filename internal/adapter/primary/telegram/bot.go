// Package telegram — driving adapter, инкапсулирует работу с Telegram (telebot.v3).
// Внешние слои (cmd/bot/main.go) знают только NewBot и Run — никаких ссылок на tele.
package telegram

import (
	"context"
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

	// runCtx — базовый ctx из Run. Пишется один раз ДО старта поллера
	// (go-statement даёт happens-before), читается track-middleware.
	runCtx context.Context
	// wg учитывает in-flight handlers: telebot запускает каждый в горутине
	// и в Stop() их НЕ дожидается — ждём сами в Run, иначе процесс выходит
	// посреди работающего handler'а.
	wg sync.WaitGroup
}

// NewBot собирает telebot, навешивает middleware в правильном порядке,
// регистрирует команды. Handlers инжектируются снаружи — это позволяет тестировать
// бота с фейк-handler'ами и переиспользовать handler'ы (например в CLI-режиме).
func NewBot(cfg Config, logger *slog.Logger, h Handlers) (*Bot, error) {
	tb, err := tele.NewBot(tele.Settings{
		Token:  cfg.Token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		return nil, fmt.Errorf("init telebot: %w", err)
	}

	b := &Bot{bot: tb, logger: logger}

	// Порядок middleware важен: сначала Auth (чтобы не логировать спам от чужих
	// и не считать его in-flight), затем track (учёт + базовый ctx), потом Log.
	tb.Use(middleware.Auth(cfg.AllowedUserIDs, logger))
	tb.Use(b.track)
	tb.Use(middleware.Log(logger))

	registerRoutes(tb, h)

	// Нативное меню команд Telegram (кнопка ≡ у поля ввода + автодополнение по «/»).
	// Некритично: если API недоступен в момент старта — бот работает и без меню,
	// поэтому ошибку логируем, но не валим запуск.
	if err := tb.SetCommands(menuCommands(h)); err != nil {
		logger.Warn("не удалось установить меню команд Telegram", "err", err)
	}

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

// Run блокирующе работает до отмены ctx. telebot.Start блокирующий, поэтому
// запускаем его в отдельной горутине и сигнализируем через done. На отмене ctx
// зовём Stop (корректно завершает long-poll-цикл) и дожидаемся in-flight handlers.
func (b *Bot) Run(ctx context.Context) error {
	b.runCtx = ctx // до go Start() — happens-before для track

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
		// telebot.Start вернулся сам по себе (фатальная ошибка long-poll).
		return fmt.Errorf("telegram bot stopped unexpectedly")
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

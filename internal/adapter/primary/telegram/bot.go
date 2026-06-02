// Package telegram — driving adapter, инкапсулирует работу с Telegram (telebot.v3).
// Внешние слои (cmd/bot/main.go) знают только NewBot и Run — никаких ссылок на tele.
package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/tumour/openwrt-bot/internal/adapter/primary/telegram/middleware"
	tele "gopkg.in/telebot.v3"
)

// Config — настройки бота. Tokенов мы тут не валидируем (это работа config layer).
type Config struct {
	Token          string
	AllowedChatIDs []int64
}

// Bot — обёртка над telebot.Bot с lifecycle-методом Run, удобным для main.
type Bot struct {
	bot    *tele.Bot
	logger *slog.Logger
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

	// Порядок middleware важен: сначала Auth (чтобы не логировать спам от чужих),
	// потом Log (логируем только прошедшие auth).
	tb.Use(middleware.Auth(cfg.AllowedChatIDs, logger))
	tb.Use(middleware.Log(logger))

	registerRoutes(tb, h)

	// Нативное меню команд Telegram (кнопка ≡ у поля ввода + автодополнение по «/»).
	// Некритично: если API недоступен в момент старта — бот работает и без меню,
	// поэтому ошибку логируем, но не валим запуск.
	if err := tb.SetCommands(menuCommands(h)); err != nil {
		logger.Warn("не удалось установить меню команд Telegram", "err", err)
	}

	return &Bot{bot: tb, logger: logger}, nil
}

// Run блокирующе работает до отмены ctx. telebot.Start блокирующий, поэтому
// запускаем его в отдельной горутине и сигнализируем через done. На отмене ctx
// зовём Stop — он корректно завершает long-poll-цикл.
func (b *Bot) Run(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		defer close(done)
		b.bot.Start()
	}()

	select {
	case <-ctx.Done():
		b.logger.Info("shutting down telegram bot")
		b.bot.Stop()
		<-done
		return nil
	case <-done:
		// telebot.Start вернулся сам по себе (фатальная ошибка long-poll).
		return fmt.Errorf("telegram bot stopped unexpectedly")
	}
}

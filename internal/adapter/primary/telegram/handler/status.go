package handler

import (
	"context"
	"fmt"

	"github.com/tumour/openwrt-bot/internal/adapter/primary/telegram/middleware"
	"github.com/tumour/openwrt-bot/internal/adapter/primary/telegram/presenter"
	"github.com/tumour/openwrt-bot/internal/app/status"
	tele "gopkg.in/telebot.v3"
)

// Status — handler для команды /status. Зависит только от use case (не от
// конкретного ubus-клиента) — это даёт замену реализации без правок здесь.
type Status struct {
	uc *status.GetStatus
}

func NewStatus(uc *status.GetStatus) *Status {
	return &Status{uc: uc}
}

// Handle — точка входа от telebot. Подпись фиксирована библиотекой:
// `func(c tele.Context) error`.
func (h *Status) Handle(c tele.Context) error {
	ctx, cancel := context.WithTimeout(middleware.BaseContext(c), handlerTimeout)
	defer cancel()

	out, err := h.uc.Execute(ctx, status.GetStatusInput{})
	if err != nil {
		_ = answer(c, "⚠ не удалось получить статус роутера")
		return fmt.Errorf("/status: %w", err)
	}
	return answer(c, presenter.Status(out.Snapshot), tele.ModeHTML)
}

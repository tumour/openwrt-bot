// Package handler — driving adapter: каждая команда Telegram превращается в
// вызов соответствующего use case. Handler НЕ содержит бизнес-логики, только
// I/O-преобразование: telebot.Context → use case Input → presenter → Send.
package handler

import (
	"context"
	"time"

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

// handlerTimeout — внутренний лимит, чтобы зависший exec не повесил бот.
// 10 секунд с запасом для самого медленного `ubus call` на слабом роутере.
const handlerTimeout = 10 * time.Second

// Handle — точка входа от telebot. Подпись фиксирована библиотекой:
// `func(c tele.Context) error`.
func (h *Status) Handle(c tele.Context) error {
	ctx, cancel := context.WithTimeout(context.Background(), handlerTimeout)
	defer cancel()

	out, err := h.uc.Execute(ctx, status.GetStatusInput{})
	if err != nil {
		return c.Send("⚠ не удалось получить статус: " + err.Error())
	}
	return c.Send(presenter.Status(out.Snapshot), tele.ModeMarkdown)
}

package handler

import (
	"context"

	"github.com/tumour/openwrt-bot/internal/adapter/primary/telegram/presenter"
	"github.com/tumour/openwrt-bot/internal/app/device"
	"github.com/tumour/openwrt-bot/internal/domain"
	tele "gopkg.in/telebot.v3"
)

// Unban — handler команды /unban <mac>. Симметричен Ban: тот же паттерн парсинга,
// валидации и вызова use case.
type Unban struct {
	uc *device.Unban
}

func NewUnban(uc *device.Unban) *Unban { return &Unban{uc: uc} }

func (h *Unban) Handle(c tele.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return c.Send("использование: `/unban aa:bb:cc:dd:ee:ff`", tele.ModeMarkdown)
	}

	mac, err := domain.NewMAC(args[0])
	if err != nil {
		return c.Send("⚠ невалидный MAC: " + args[0])
	}

	ctx, cancel := context.WithTimeout(context.Background(), handlerTimeout)
	defer cancel()

	if _, err := h.uc.Execute(ctx, device.UnbanInput{MAC: mac}); err != nil {
		return c.Send("⚠ " + err.Error())
	}
	return c.Send(presenter.Unbanned(mac), tele.ModeMarkdown)
}

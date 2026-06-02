package handler

import (
	"context"

	"github.com/tumour/openwrt-bot/internal/adapter/primary/telegram/presenter"
	"github.com/tumour/openwrt-bot/internal/app/device"
	tele "gopkg.in/telebot.v3"
)

// List — handler команды /list. Без аргументов: показывает все DHCP-лизы
// с пометкой "забанен".
type List struct {
	uc *device.List
}

func NewList(uc *device.List) *List { return &List{uc: uc} }

func (h *List) Handle(c tele.Context) error {
	ctx, cancel := context.WithTimeout(context.Background(), handlerTimeout)
	defer cancel()

	out, err := h.uc.Execute(ctx, device.ListInput{})
	if err != nil {
		return c.Send("⚠ " + err.Error())
	}
	return c.Send(presenter.DeviceList(out.Devices), tele.ModeHTML)
}

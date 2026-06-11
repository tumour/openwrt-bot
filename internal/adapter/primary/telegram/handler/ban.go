package handler

import (
	"context"
	"fmt"

	"github.com/tumour/openwrt-bot/internal/adapter/primary/telegram/presenter"
	"github.com/tumour/openwrt-bot/internal/app/device"
	"github.com/tumour/openwrt-bot/internal/domain"
	tele "gopkg.in/telebot.v3"
)

// Ban — handler команды /ban <mac>. Парсит аргумент, валидирует через value object,
// вызывает use case. Сам не знает ни о nftables, ни о форме ответа Telegram.
type Ban struct {
	uc *device.Ban
}

func NewBan(uc *device.Ban) *Ban { return &Ban{uc: uc} }

func (h *Ban) Handle(c tele.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return c.Send("использование: `/ban aa:bb:cc:dd:ee:ff`", tele.ModeMarkdown)
	}

	mac, err := domain.NewMAC(args[0])
	if err != nil {
		return c.Send("⚠ невалидный MAC: " + args[0])
	}

	ctx, cancel := context.WithTimeout(context.Background(), handlerTimeout)
	defer cancel()

	if _, err := h.uc.Execute(ctx, device.BanInput{MAC: mac}); err != nil {
		_ = c.Send("⚠ не удалось забанить устройство")
		return fmt.Errorf("/ban: %w", err)
	}
	return c.Send(presenter.Banned(mac), tele.ModeMarkdown)
}

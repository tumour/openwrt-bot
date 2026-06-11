package handler

import (
	"context"
	"fmt"

	"github.com/tumour/openwrt-bot/internal/adapter/primary/telegram/middleware"
	"github.com/tumour/openwrt-bot/internal/adapter/primary/telegram/presenter"
	"github.com/tumour/openwrt-bot/internal/app/device"
	"github.com/tumour/openwrt-bot/internal/domain"
	tele "gopkg.in/telebot.v3"
)

// VPNOff — handler команды /vpnoff <mac>: пустить устройство мимо VPN.
// Тот же паттерн, что Ban: парсинг → value object → use case.
type VPNOff struct {
	uc *device.DisableVPN
}

func NewVPNOff(uc *device.DisableVPN) *VPNOff { return &VPNOff{uc: uc} }

func (h *VPNOff) Handle(c tele.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return c.Send("использование: <code>/vpnoff aa:bb:cc:dd:ee:ff</code>", tele.ModeHTML)
	}

	mac, err := domain.NewMAC(args[0])
	if err != nil {
		return c.Send("⚠ невалидный MAC: " + args[0])
	}

	ctx, cancel := context.WithTimeout(middleware.BaseContext(c), handlerTimeout)
	defer cancel()

	if _, err := h.uc.Execute(ctx, device.DisableVPNInput{MAC: mac}); err != nil {
		_ = c.Send("⚠ не удалось вывести устройство из VPN")
		return fmt.Errorf("/vpnoff: %w", err)
	}
	return c.Send(presenter.VPNOff(mac), tele.ModeHTML)
}

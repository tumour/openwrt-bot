package handler

import (
	"context"
	"fmt"

	"github.com/tumour/openwrt-bot/internal/adapter/primary/telegram/presenter"
	"github.com/tumour/openwrt-bot/internal/app/device"
	"github.com/tumour/openwrt-bot/internal/domain"
	tele "gopkg.in/telebot.v3"
)

// VPNOn — handler команды /vpnon <mac>: вернуть устройство в VPN
// (убрать из сета обхода). Симметричен VPNOff.
type VPNOn struct {
	uc *device.EnableVPN
}

func NewVPNOn(uc *device.EnableVPN) *VPNOn { return &VPNOn{uc: uc} }

func (h *VPNOn) Handle(c tele.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return c.Send("использование: <code>/vpnon aa:bb:cc:dd:ee:ff</code>", tele.ModeHTML)
	}

	mac, err := domain.NewMAC(args[0])
	if err != nil {
		return c.Send("⚠ невалидный MAC: " + args[0])
	}

	ctx, cancel := context.WithTimeout(context.Background(), handlerTimeout)
	defer cancel()

	if _, err := h.uc.Execute(ctx, device.EnableVPNInput{MAC: mac}); err != nil {
		_ = c.Send("⚠ не удалось вернуть устройство в VPN")
		return fmt.Errorf("/vpnon: %w", err)
	}
	return c.Send(presenter.VPNOn(mac), tele.ModeHTML)
}

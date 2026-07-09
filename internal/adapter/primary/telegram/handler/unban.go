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

// Unban — handler команды /unban <mac>. Симметричен Ban: тот же паттерн парсинга,
// валидации и вызова use case.
type Unban struct {
	uc *device.Unban
}

func NewUnban(uc *device.Unban) *Unban { return &Unban{uc: uc} }

func (h *Unban) Handle(c tele.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return answer(c, "использование: <code>/unban aa:bb:cc:dd:ee:ff</code>", tele.ModeHTML)
	}

	mac, err := domain.NewMAC(args[0])
	if err != nil {
		return answer(c, "⚠ невалидный MAC: "+args[0])
	}

	ctx, cancel := context.WithTimeout(middleware.BaseContext(c), handlerTimeout)
	defer cancel()

	if _, err := h.uc.Execute(ctx, device.UnbanInput{MAC: mac}); err != nil {
		_ = answer(c, "⚠ не удалось разбанить устройство")
		return fmt.Errorf("/unban: %w", err)
	}
	return answer(c, presenter.Unbanned(mac), tele.ModeHTML)
}

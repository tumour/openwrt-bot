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

// Unlimit — handler команды /unlimit <mac>. Симметричен Unban; снятие
// отсутствующего лимита — такой же молчаливый успех (идемпотентный порт).
type Unlimit struct {
	uc *device.RemoveLimit
}

func NewUnlimit(uc *device.RemoveLimit) *Unlimit { return &Unlimit{uc: uc} }

func (h *Unlimit) Handle(c tele.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return answer(c, "использование: <code>/unlimit aa:bb:cc:dd:ee:ff</code>", tele.ModeHTML)
	}

	mac, err := domain.NewMAC(args[0])
	if err != nil {
		return answer(c, "⚠ невалидный MAC: "+args[0])
	}

	ctx, cancel := context.WithTimeout(middleware.BaseContext(c), handlerTimeout)
	defer cancel()

	if _, err := h.uc.Execute(ctx, device.RemoveLimitInput{MAC: mac}); err != nil {
		_ = answer(c, "⚠ не удалось снять лимит скорости")
		return fmt.Errorf("/unlimit: %w", err)
	}
	return answer(c, presenter.Unlimited(mac), tele.ModeHTML)
}

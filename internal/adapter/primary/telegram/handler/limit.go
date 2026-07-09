package handler

import (
	"context"
	"fmt"
	"strconv"

	"github.com/tumour/openwrt-bot/internal/adapter/primary/telegram/middleware"
	"github.com/tumour/openwrt-bot/internal/adapter/primary/telegram/presenter"
	"github.com/tumour/openwrt-bot/internal/app/device"
	"github.com/tumour/openwrt-bot/internal/domain"
	tele "gopkg.in/telebot.v3"
)

// Limit — handler команды /limit <mac> <КБ/с>. Тот же паттерн, что Ban, плюс
// второй аргумент: число валидируется через value object domain.Rate.
type Limit struct {
	uc *device.SetLimit
}

func NewLimit(uc *device.SetLimit) *Limit { return &Limit{uc: uc} }

func (h *Limit) Handle(c tele.Context) error {
	args := c.Args()
	if len(args) < 2 {
		return answer(c, "использование: <code>/limit aa:bb:cc:dd:ee:ff 512</code> — КБ/с на каждое направление", tele.ModeHTML)
	}

	mac, err := domain.NewMAC(args[0])
	if err != nil {
		return answer(c, "⚠ невалидный MAC: "+args[0])
	}
	n, err := strconv.Atoi(args[1])
	if err != nil {
		return answer(c, "⚠ невалидный лимит (целое, КБ/с): "+args[1])
	}
	rate, err := domain.NewRate(n)
	if err != nil {
		return answer(c, "⚠ невалидный лимит (1..1000000 КБ/с): "+args[1])
	}

	ctx, cancel := context.WithTimeout(middleware.BaseContext(c), handlerTimeout)
	defer cancel()

	if _, err := h.uc.Execute(ctx, device.SetLimitInput{MAC: mac, Rate: rate}); err != nil {
		_ = answer(c, "⚠ не удалось ограничить скорость")
		return fmt.Errorf("/limit: %w", err)
	}
	return answer(c, presenter.Limited(mac, rate), tele.ModeHTML)
}

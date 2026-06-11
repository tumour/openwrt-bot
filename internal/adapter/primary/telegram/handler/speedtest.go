package handler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tumour/openwrt-bot/internal/adapter/primary/telegram/middleware"
	"github.com/tumour/openwrt-bot/internal/adapter/primary/telegram/presenter"
	"github.com/tumour/openwrt-bot/internal/app/network"
	tele "gopkg.in/telebot.v3"
)

// SpeedTest — handler для /speedtest. Отличается от остальных двумя вещами:
//  1. Замер длится десятки секунд → свой таймаут много больше общего handlerTimeout.
//  2. Сразу шлём «⏳ замеряю…» и редактируем ЭТО ЖЕ сообщение результатом — чтобы
//     пользователь видел, что бот не завис, и не плодить два сообщения.
type SpeedTest struct {
	uc *network.RunSpeedTest
}

func NewSpeedTest(uc *network.RunSpeedTest) *SpeedTest {
	return &SpeedTest{uc: uc}
}

// speedtestTimeout — потолок на весь замер (download + upload + ping). librespeed
// обычно укладывается в 30-60с; берём с запасом.
const speedtestTimeout = 90 * time.Second

func (h *SpeedTest) Handle(c tele.Context) error {
	msg, err := c.Bot().Send(c.Recipient(), "⏳ Замеряю скорость канала, ~30–60 с…")
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(middleware.BaseContext(c), speedtestTimeout)
	defer cancel()

	out, err := h.uc.Execute(ctx, network.RunSpeedTestInput{})
	if err != nil {
		text := "⚠ не удалось замерить скорость"
		if errors.Is(err, network.ErrToolMissing) {
			text = "⚠ librespeed-cli не установлен на роутере — поставь: apk add librespeed-cli"
		}
		_, _ = c.Bot().Edit(msg, text)
		return fmt.Errorf("/speedtest: %w", err)
	}
	_, err = c.Bot().Edit(msg, presenter.SpeedTest(out.Result), tele.ModeHTML)
	return err
}

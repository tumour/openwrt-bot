package handler

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/tumour/openwrt-bot/internal/adapter/primary/telegram/middleware"
	"github.com/tumour/openwrt-bot/internal/adapter/primary/telegram/presenter"
	"github.com/tumour/openwrt-bot/internal/app/device"
	"github.com/tumour/openwrt-bot/internal/domain"
	tele "gopkg.in/telebot.v3"
)

// Devices — интерактивный /list: устройства inline-кнопками, тап открывает
// карточку с действиями (бан/разбан, vpn вкл/выкл, лимит скорости) по текущему
// состоянию. Команды /ban, /vpnoff, /limit и т.д. остаются как быстрый ручной
// путь — здесь используются те же use cases.
type Devices struct {
	list        *device.List
	ban         *device.Ban
	unban       *device.Unban
	vpnOff      *device.DisableVPN
	vpnOn       *device.EnableVPN
	setLimit    *device.SetLimit
	removeLimit *device.RemoveLimit
}

func NewDevices(list *device.List, ban *device.Ban, unban *device.Unban,
	vpnOff *device.DisableVPN, vpnOn *device.EnableVPN,
	setLimit *device.SetLimit, removeLimit *device.RemoveLimit) *Devices {
	return &Devices{list: list, ban: ban, unban: unban, vpnOff: vpnOff, vpnOn: vpnOn,
		setLimit: setLimit, removeLimit: removeLimit}
}

// Уникальные id callback-кнопок (telebot матчит обработчик по unique,
// payload — MAC устройства; у пресетов лимита — "mac|rate").
const (
	cbCard    = "card" // открыть/обновить карточку
	cbBack    = "back" // вернуться к списку
	cbBan     = "ban"  // действия из карточки ↓
	cbUnban   = "unban"
	cbVPNOff  = "vpnoff"
	cbVPNOn   = "vpnon"
	cbLimit   = "limit"
	cbUnlimit = "unlimit"
)

// limitPresets — кнопки-пресеты лимита в карточке, КБ/с на каждое направление.
// Произвольные значения — текстом: /limit <MAC> <КБ/с>.
var limitPresets = []int{256, 512, 1024, 2048}

// RegisterCallbacks вешает обработчики inline-кнопок. Вызывается из router'а
// рядом с регистрацией команд; middleware (Auth/Log) применяются и к callback'ам.
func (h *Devices) RegisterCallbacks(bot *tele.Bot) {
	bot.Handle(&tele.Btn{Unique: cbCard}, h.handleCard)
	bot.Handle(&tele.Btn{Unique: cbBack}, h.handleBack)
	bot.Handle(&tele.Btn{Unique: cbBan}, h.action(func(ctx context.Context, mac domain.MAC) error {
		_, err := h.ban.Execute(ctx, device.BanInput{MAC: mac})
		return err
	}, "🚫 забанено"))
	bot.Handle(&tele.Btn{Unique: cbUnban}, h.action(func(ctx context.Context, mac domain.MAC) error {
		_, err := h.unban.Execute(ctx, device.UnbanInput{MAC: mac})
		return err
	}, "🟢 разбанено"))
	bot.Handle(&tele.Btn{Unique: cbVPNOff}, h.action(func(ctx context.Context, mac domain.MAC) error {
		_, err := h.vpnOff.Execute(ctx, device.DisableVPNInput{MAC: mac})
		return err
	}, "🌐 ходит напрямую"))
	bot.Handle(&tele.Btn{Unique: cbVPNOn}, h.action(func(ctx context.Context, mac domain.MAC) error {
		_, err := h.vpnOn.Execute(ctx, device.EnableVPNInput{MAC: mac})
		return err
	}, "🔒 снова через VPN"))
	bot.Handle(&tele.Btn{Unique: cbUnlimit}, h.action(func(ctx context.Context, mac domain.MAC) error {
		_, err := h.removeLimit.Execute(ctx, device.RemoveLimitInput{MAC: mac})
		return err
	}, "♾ лимит снят"))
	bot.Handle(&tele.Btn{Unique: cbLimit}, h.handleLimit)
}

// HandleList — команда /list: заголовок + кнопка на каждое устройство.
func (h *Devices) HandleList(c tele.Context) error {
	ctx, cancel := context.WithTimeout(middleware.BaseContext(c), handlerTimeout)
	defer cancel()

	out, err := h.list.Execute(ctx, device.ListInput{})
	if err != nil {
		_ = answer(c, "⚠ не удалось получить список устройств")
		return fmt.Errorf("/list: %w", err)
	}
	return c.Send(presenter.ListHeader(len(out.Devices)), listMarkup(out.Devices), tele.ModeHTML)
}

// handleCard — тап по устройству (или «Обновить»): редактируем сообщение в карточку.
func (h *Devices) handleCard(c tele.Context) error {
	mac, err := domain.NewMAC(c.Data())
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "⚠ битый MAC в кнопке"})
	}
	if err := h.renderCard(c, mac); err != nil {
		_ = c.Respond(&tele.CallbackResponse{Text: "⚠ не получилось"})
		return fmt.Errorf("card %s: %w", mac, err)
	}
	return c.Respond()
}

// handleBack — «К списку»: редактируем карточку обратно в список.
func (h *Devices) handleBack(c tele.Context) error {
	ctx, cancel := context.WithTimeout(middleware.BaseContext(c), handlerTimeout)
	defer cancel()

	out, err := h.list.Execute(ctx, device.ListInput{})
	if err != nil {
		_ = c.Respond(&tele.CallbackResponse{Text: "⚠ не получилось"})
		return fmt.Errorf("back to list: %w", err)
	}
	if err := editKeepalive(c, presenter.ListHeader(len(out.Devices)), listMarkup(out.Devices)); err != nil {
		return err
	}
	return c.Respond()
}

// action — обёртка для кнопок-действий карточки: выполнить use case,
// перерисовать карточку по свежему состоянию, показать toast.
func (h *Devices) action(do func(context.Context, domain.MAC) error, toast string) tele.HandlerFunc {
	return func(c tele.Context) error {
		mac, err := domain.NewMAC(c.Data())
		if err != nil {
			return c.Respond(&tele.CallbackResponse{Text: "⚠ битый MAC в кнопке"})
		}

		ctx, cancel := context.WithTimeout(middleware.BaseContext(c), handlerTimeout)
		defer cancel()

		if err := do(ctx, mac); err != nil {
			_ = c.Respond(&tele.CallbackResponse{Text: "⚠ не получилось"})
			return fmt.Errorf("card action %s: %w", mac, err)
		}
		if err := h.renderCard(c, mac); err != nil {
			return fmt.Errorf("refresh card %s: %w", mac, err)
		}
		return c.Respond(&tele.CallbackResponse{Text: toast})
	}
}

// handleLimit — тап по кнопке-пресету: как action(), но payload двойной
// ("mac|rate"), поэтому парсинг свой.
func (h *Devices) handleLimit(c tele.Context) error {
	mac, rate, err := parseLimitPayload(c.Data())
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "⚠ битая кнопка лимита"})
	}

	ctx, cancel := context.WithTimeout(middleware.BaseContext(c), handlerTimeout)
	defer cancel()

	if _, err := h.setLimit.Execute(ctx, device.SetLimitInput{MAC: mac, Rate: rate}); err != nil {
		_ = c.Respond(&tele.CallbackResponse{Text: "⚠ не получилось"})
		return fmt.Errorf("card limit %s: %w", mac, err)
	}
	if err := h.renderCard(c, mac); err != nil {
		return fmt.Errorf("refresh card %s: %w", mac, err)
	}
	return c.Respond(&tele.CallbackResponse{Text: "⏱ лимит " + rate.String() + " КБ/с"})
}

// parseLimitPayload разбирает payload кнопки-пресета "mac|rate" (так telebot
// склеивает варargs в m.Data). Всё через value objects: битая кнопка должна
// давать toast, а не панику.
func parseLimitPayload(data string) (domain.MAC, domain.Rate, error) {
	macStr, rateStr, ok := strings.Cut(data, "|")
	if !ok {
		return "", 0, fmt.Errorf("limit payload без разделителя: %q", data)
	}
	mac, err := domain.NewMAC(macStr)
	if err != nil {
		return "", 0, err
	}
	n, err := strconv.Atoi(rateStr)
	if err != nil {
		return "", 0, fmt.Errorf("limit payload: rate не число: %q", rateStr)
	}
	rate, err := domain.NewRate(n)
	if err != nil {
		return "", 0, err
	}
	return mac, rate, nil
}

// renderCard рисует карточку устройства по свежему состоянию (Edit сообщения).
func (h *Devices) renderCard(c tele.Context, mac domain.MAC) error {
	ctx, cancel := context.WithTimeout(middleware.BaseContext(c), handlerTimeout)
	defer cancel()

	out, err := h.list.Execute(ctx, device.ListInput{})
	if err != nil {
		return err
	}
	for _, v := range out.Devices {
		if v.Device.MAC == mac {
			return editKeepalive(c, presenter.DeviceCard(v), cardMarkup(v))
		}
	}
	// Устройство ушло из DHCP-лиз между /list и тапом.
	return editKeepalive(c, presenter.CardGone(mac.String()), backOnlyMarkup())
}

// editKeepalive — Edit, игнорирующий «message is not modified»: повторный тап
// «Обновить» без изменений — это не ошибка. Сначала типизированные ошибки
// telebot (Telegram шлёт два варианта описания), подстрока — страховка на
// случай, если Telegram поменяет текст и маппинг telebot перестанет узнавать.
func editKeepalive(c tele.Context, text string, markup *tele.ReplyMarkup) error {
	err := c.Edit(text, markup, tele.ModeHTML)
	if errors.Is(err, tele.ErrSameMessageContent) || errors.Is(err, tele.ErrMessageNotModified) {
		return nil
	}
	if err != nil && strings.Contains(err.Error(), "not modified") {
		return nil
	}
	return err
}

// listMarkup — кнопка на устройство, по одной в ряд (имена длинные).
func listMarkup(views []device.View) *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	rows := make([]tele.Row, 0, len(views))
	for _, v := range views {
		rows = append(rows, m.Row(m.Data(presenter.DeviceLabel(v), cbCard, v.Device.MAC.String())))
	}
	m.Inline(rows...)
	return m
}

// cardMarkup — действия по текущему состоянию: показываем только осмысленный
// тумблер (забаненному — «Разбанить», не забаненному — «Забанить»). Ряд
// пресетов лимита (КБ/с — единицы объясняет строка «Лимит» в тексте карточки)
// виден всегда, активный пресет помечен ✓; «Снять лимит» — только когда есть что снимать.
func cardMarkup(v device.View) *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	mac := v.Device.MAC.String()

	banBtn := m.Data("🚫 Забанить", cbBan, mac)
	if v.Banned {
		banBtn = m.Data("🟢 Разбанить", cbUnban, mac)
	}
	vpnBtn := m.Data("🌐 Выключить VPN", cbVPNOff, mac)
	if v.Direct {
		vpnBtn = m.Data("🔒 Включить VPN", cbVPNOn, mac)
	}
	presets := make([]tele.Btn, 0, len(limitPresets))
	for _, p := range limitPresets {
		label := strconv.Itoa(p)
		if v.Limit.KBps() == p {
			label = "✓ " + label
		}
		presets = append(presets, m.Data(label, cbLimit, mac, strconv.Itoa(p)))
	}

	rows := []tele.Row{
		m.Row(banBtn, vpnBtn),
		m.Row(presets...),
	}
	if !v.Limit.IsZero() {
		rows = append(rows, m.Row(m.Data("♾ Снять лимит", cbUnlimit, mac)))
	}
	rows = append(rows, m.Row(m.Data("⬅ К списку", cbBack), m.Data("🔄 Обновить", cbCard, mac)))
	m.Inline(rows...)
	return m
}

func backOnlyMarkup() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	m.Inline(m.Row(m.Data("⬅ К списку", cbBack)))
	return m
}

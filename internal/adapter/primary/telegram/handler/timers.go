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
	"github.com/tumour/openwrt-bot/internal/app/timer"
	"github.com/tumour/openwrt-bot/internal/domain"
	tele "gopkg.in/telebot.v3"
)

// Timers — фича «⏰ Таймеры»: отложенные бан/разбан/vpn по расписанию. Флоу
// stateless, всё состояние выбора кодируется в payload inline-кнопок (как в
// карточках /list): устройство → действие → задержка. Точное значение задержки —
// через текст-команду /timer. Действия при срабатывании выполняют те же use
// cases device.*, что и кнопки карточек — связка в composition root.
type Timers struct {
	schedule *timer.Schedule
	timers   *timer.List
	cancel   *timer.Cancel
	devices  *device.List
}

func NewTimers(schedule *timer.Schedule, timers *timer.List, cancel *timer.Cancel, devices *device.List) *Timers {
	return &Timers{schedule: schedule, timers: timers, cancel: cancel, devices: devices}
}

// Уникальные id callback-кнопок флоу таймеров (telebot матчит обработчик по
// unique; payload склеивается через "|").
const (
	cbTimerRoot   = "tmroot"   // корневой экран (список активных + устройства)
	cbTimerPick   = "tmpick"   // выбрали устройство → экран действий; payload = mac
	cbTimerAction = "tmact"    // выбрали действие → экран задержек; payload = mac|action
	cbTimerSet    = "tmset"    // выбрали задержку → поставить; payload = mac|action|minutes
	cbTimerCancel = "tmcancel" // отменить таймер; payload = taskID
)

// minutePresets — кнопки-пресеты задержки, минуты. Произвольное значение —
// текст-командой /timer <MAC> <действие> <минуты>.
var minutePresets = []int{5, 15, 30, 60, 120, 240}

// RegisterCallbacks вешает обработчики inline-кнопок таймеров. Вызывается из
// router'а рядом с регистрацией команд; middleware (Auth/Log) применяются и здесь.
func (h *Timers) RegisterCallbacks(bot *tele.Bot) {
	bot.Handle(&tele.Btn{Unique: cbTimerRoot}, h.handleRoot)
	bot.Handle(&tele.Btn{Unique: cbTimerPick}, h.handlePick)
	bot.Handle(&tele.Btn{Unique: cbTimerAction}, h.handleAction)
	bot.Handle(&tele.Btn{Unique: cbTimerSet}, h.handleSet)
	bot.Handle(&tele.Btn{Unique: cbTimerCancel}, h.handleCancel)
}

// HandleRoot — вход по кнопке «⏰ Таймеры» / команде /timers: новое сообщение с
// активными таймерами и списком устройств.
func (h *Timers) HandleRoot(c tele.Context) error {
	ctx, cancel := context.WithTimeout(middleware.BaseContext(c), handlerTimeout)
	defer cancel()

	text, markup, err := h.renderRoot(ctx)
	if err != nil {
		_ = answer(c, "⚠ не удалось открыть таймеры")
		return fmt.Errorf("/timers: %w", err)
	}
	return c.Send(text, markup, tele.ModeHTML)
}

// handleRoot — тот же корневой экран через callback («Обновить», «Назад к списку»):
// редактируем текущее сообщение вместо отправки нового.
func (h *Timers) handleRoot(c tele.Context) error {
	ctx, cancel := context.WithTimeout(middleware.BaseContext(c), handlerTimeout)
	defer cancel()

	text, markup, err := h.renderRoot(ctx)
	if err != nil {
		_ = c.Respond(&tele.CallbackResponse{Text: "⚠ не получилось"})
		return fmt.Errorf("timers root: %w", err)
	}
	if err := editKeepalive(c, text, markup); err != nil {
		return err
	}
	return c.Respond()
}

// handlePick — тап по устройству: экран выбора действия.
func (h *Timers) handlePick(c tele.Context) error {
	mac, err := domain.NewMAC(c.Data())
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "⚠ битый MAC в кнопке"})
	}
	ctx, cancel := context.WithTimeout(middleware.BaseContext(c), handlerTimeout)
	defer cancel()

	if err := editKeepalive(c, presenter.TimerActionPrompt(h.deviceName(ctx, mac)), actionMarkup(mac)); err != nil {
		_ = c.Respond(&tele.CallbackResponse{Text: "⚠ не получилось"})
		return fmt.Errorf("timer pick %s: %w", mac, err)
	}
	return c.Respond()
}

// handleAction — тап по действию: экран выбора задержки.
func (h *Timers) handleAction(c tele.Context) error {
	mac, action, err := parseTimerAction(c.Data())
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "⚠ битая кнопка"})
	}
	ctx, cancel := context.WithTimeout(middleware.BaseContext(c), handlerTimeout)
	defer cancel()

	if err := editKeepalive(c, presenter.TimerMinutesPrompt(h.deviceName(ctx, mac), action), minutesMarkup(mac, action)); err != nil {
		_ = c.Respond(&tele.CallbackResponse{Text: "⚠ не получилось"})
		return fmt.Errorf("timer action %s: %w", mac, err)
	}
	return c.Respond()
}

// handleSet — тап по пресету задержки: поставить таймер, вернуться к корню
// (новый таймер уже в списке), показать toast.
func (h *Timers) handleSet(c tele.Context) error {
	mac, action, mins, err := parseTimerSet(c.Data())
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "⚠ битая кнопка"})
	}
	ctx, cancel := context.WithTimeout(middleware.BaseContext(c), handlerTimeout)
	defer cancel()

	if _, err := h.schedule.Execute(ctx, timer.ScheduleInput{MAC: mac, Action: action, Delay: mins}); err != nil {
		_ = c.Respond(&tele.CallbackResponse{Text: "⚠ не удалось поставить"})
		return fmt.Errorf("timer set %s: %w", mac, err)
	}
	// Таймер уже стоит — toast обязан дойти даже при упавшей перерисовке.
	return h.respondAndRefreshRoot(ctx, c, "⏰ запланировано: "+presenter.RemainingText(mins.Duration()))
}

// handleCancel — тап по «✖» активного таймера: снять и перерисовать корень.
// Уже неактивный таймер (кнопка из старого сообщения) — не ошибка, свой toast.
func (h *Timers) handleCancel(c tele.Context) error {
	id, err := parseTaskID(c.Data())
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "⚠ битая кнопка"})
	}
	ctx, cancel := context.WithTimeout(middleware.BaseContext(c), handlerTimeout)
	defer cancel()

	toast := "✖ отменено"
	if _, err := h.cancel.Execute(ctx, timer.CancelInput{ID: id}); err != nil {
		if !errors.Is(err, domain.ErrTaskNotFound) {
			_ = c.Respond(&tele.CallbackResponse{Text: "⚠ не получилось"})
			return fmt.Errorf("timer cancel %s: %w", id, err)
		}
		toast = "таймер уже неактивен"
	}
	return h.respondAndRefreshRoot(ctx, c, toast)
}

// HandleSchedule — команда /timer <MAC> <действие> <минуты>: точное значение
// задержки текстом. Кнопки-пресеты покрывают частые случаи, это — для остальных.
func (h *Timers) HandleSchedule(c tele.Context) error {
	args := c.Args()
	if len(args) < 3 {
		return answer(c, "использование: <code>/timer aa:bb:cc:dd:ee:ff ban 45</code>\n"+
			"действия: <code>ban unban vpnoff vpnon</code>, минуты 1..1440", tele.ModeHTML)
	}
	mac, err := domain.NewMAC(args[0])
	if err != nil {
		return answer(c, "⚠ невалидный MAC: "+args[0])
	}
	action, err := domain.ParseAction(args[1])
	if err != nil {
		return answer(c, "⚠ неизвестное действие: "+args[1]+" (ban|unban|vpnoff|vpnon)")
	}
	n, err := strconv.Atoi(args[2])
	if err != nil {
		return answer(c, "⚠ невалидные минуты (целое): "+args[2])
	}
	mins, err := domain.NewMinutes(n)
	if err != nil {
		return answer(c, "⚠ невалидные минуты (1..1440): "+args[2])
	}

	ctx, cancel := context.WithTimeout(middleware.BaseContext(c), handlerTimeout)
	defer cancel()
	if _, err := h.schedule.Execute(ctx, timer.ScheduleInput{MAC: mac, Action: action, Delay: mins}); err != nil {
		_ = answer(c, "⚠ не удалось поставить таймер")
		return fmt.Errorf("/timer: %w", err)
	}
	return answer(c, presenter.Scheduled(mac, action, mins), tele.ModeHTML)
}

// respondAndRefreshRoot — финал действия (постановка/отмена): перерисовать
// корневой экран и показать toast. Действие уже совершено, поэтому toast
// уходит и при упавшей перерисовке — а её ошибка идёт наверх в лог.
func (h *Timers) respondAndRefreshRoot(ctx context.Context, c tele.Context, toast string) error {
	text, markup, err := h.renderRoot(ctx)
	if err == nil {
		err = editKeepalive(c, text, markup)
	}
	if err != nil {
		_ = c.Respond(&tele.CallbackResponse{Text: toast})
		return fmt.Errorf("timers refresh: %w", err)
	}
	return c.Respond(&tele.CallbackResponse{Text: toast})
}

// renderRoot собирает корневой экран: активные таймеры (строки + кнопки отмены)
// и устройства для новых таймеров. Имена устройств берём один раз из /list.
func (h *Timers) renderRoot(ctx context.Context) (string, *tele.ReplyMarkup, error) {
	out, err := h.devices.Execute(ctx, device.ListInput{})
	if err != nil {
		return "", nil, fmt.Errorf("list devices: %w", err)
	}
	tasks, err := h.timers.Execute(ctx, timer.ListInput{})
	if err != nil {
		return "", nil, fmt.Errorf("list timers: %w", err)
	}
	names := deviceNames(out.Devices)
	return presenter.Timers(tasks.Tasks, names), rootMarkup(out.Devices, tasks.Tasks, names), nil
}

// deviceName — имя устройства по MAC (свежий /list); ушло из DHCP — сам MAC.
func (h *Timers) deviceName(ctx context.Context, mac domain.MAC) string {
	out, err := h.devices.Execute(ctx, device.ListInput{})
	if err != nil {
		return mac.String()
	}
	for _, v := range out.Devices {
		if v.Device.MAC == mac {
			return presenter.DeviceName(v)
		}
	}
	return mac.String()
}

// deviceNames — MAC → отображаемое имя, для строк активных таймеров.
func deviceNames(views []device.View) map[domain.MAC]string {
	names := make(map[domain.MAC]string, len(views))
	for _, v := range views {
		names[v.Device.MAC] = presenter.DeviceName(v)
	}
	return names
}

// rootMarkup — кнопки отмены активных таймеров (по одной на таймер), затем
// устройства для нового таймера, затем «Обновить».
func rootMarkup(views []device.View, tasks []timer.View, names map[domain.MAC]string) *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	rows := make([]tele.Row, 0, len(tasks)+len(views)+1)
	for _, tv := range tasks {
		label := fmt.Sprintf("✖ %s · %s", presenter.DeviceNameOr(names, tv.Job.MAC), presenter.ActionLabel(tv.Job.Action))
		rows = append(rows, m.Row(m.Data(label, cbTimerCancel, tv.ID.String())))
	}
	for _, v := range views {
		rows = append(rows, m.Row(m.Data("📱 "+presenter.DeviceName(v), cbTimerPick, v.Device.MAC.String())))
	}
	rows = append(rows, m.Row(m.Data("🔄 Обновить", cbTimerRoot)))
	m.Inline(rows...)
	return m
}

// actionMarkup — четыре действия для устройства (payload mac|action) + назад к списку.
func actionMarkup(mac domain.MAC) *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	macStr := mac.String()
	btn := func(a domain.Action) tele.Btn {
		return m.Data(presenter.ActionButton(a), cbTimerAction, macStr, a.String())
	}
	m.Inline(
		m.Row(btn(domain.ActionBan), btn(domain.ActionUnban)),
		m.Row(btn(domain.ActionVPNOff), btn(domain.ActionVPNOn)),
		m.Row(m.Data("⬅ К списку", cbTimerRoot)),
	)
	return m
}

// minutesMarkup — пресеты задержки (payload mac|action|minutes) по 3 в ряд +
// назад к выбору действия (тот же экран, что открывает cbTimerPick).
func minutesMarkup(mac domain.MAC, action domain.Action) *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	macStr, actStr := mac.String(), action.String()
	presets := make([]tele.Btn, 0, len(minutePresets))
	for _, mins := range minutePresets {
		presets = append(presets, m.Data(presenter.MinutesLabel(mins), cbTimerSet, macStr, actStr, strconv.Itoa(mins)))
	}
	rows := make([]tele.Row, 0, len(presets)/3+2)
	for i := 0; i < len(presets); i += 3 {
		end := i + 3
		if end > len(presets) {
			end = len(presets)
		}
		rows = append(rows, m.Row(presets[i:end]...))
	}
	rows = append(rows, m.Row(m.Data("⬅ Назад", cbTimerPick, macStr)))
	m.Inline(rows...)
	return m
}

// parseTimerAction разбирает payload "mac|action" (экран выбора задержки).
func parseTimerAction(data string) (domain.MAC, domain.Action, error) {
	macStr, actStr, ok := strings.Cut(data, "|")
	if !ok {
		return "", 0, fmt.Errorf("timer action payload без разделителя: %q", data)
	}
	mac, err := domain.NewMAC(macStr)
	if err != nil {
		return "", 0, err
	}
	action, err := domain.ParseAction(actStr)
	if err != nil {
		return "", 0, err
	}
	return mac, action, nil
}

// parseTimerSet разбирает payload "mac|action|minutes" (постановка таймера).
func parseTimerSet(data string) (domain.MAC, domain.Action, domain.Minutes, error) {
	parts := strings.SplitN(data, "|", 3)
	if len(parts) != 3 {
		return "", 0, 0, fmt.Errorf("timer set payload: ждём mac|action|minutes, got %q", data)
	}
	mac, err := domain.NewMAC(parts[0])
	if err != nil {
		return "", 0, 0, err
	}
	action, err := domain.ParseAction(parts[1])
	if err != nil {
		return "", 0, 0, err
	}
	n, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", 0, 0, fmt.Errorf("timer set payload: minutes не число: %q", parts[2])
	}
	mins, err := domain.NewMinutes(n)
	if err != nil {
		return "", 0, 0, err
	}
	return mac, action, mins, nil
}

// parseTaskID разбирает payload кнопки отмены (id задачи).
func parseTaskID(data string) (domain.TaskID, error) {
	n, err := strconv.ParseUint(data, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("timer id payload %q: %w", data, err)
	}
	return domain.TaskID(n), nil
}

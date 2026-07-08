package presenter

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/tumour/openwrt-bot/internal/app/timer"
	"github.com/tumour/openwrt-bot/internal/domain"
)

// Экран таймеров. Как и весь бот — только HTML-режим; имена устройств приходят
// из DHCP (внешние строки) и экранируются, MAC и числа — value objects, им нечего
// экранировать.

// actionInfo — единый источник представления действий: иконка, фраза для строк
// (нижний регистр, середина предложения) и подпись кнопки. Одна таблица вместо
// параллельных switch: новое действие добавляется одной строкой.
var actionInfo = map[domain.Action]struct {
	emoji  string
	label  string
	button string
}{
	domain.ActionBan:    {"🚫", "выключить интернет", "🚫 Выключить интернет"},
	domain.ActionUnban:  {"🟢", "включить интернет", "🟢 Включить интернет"},
	domain.ActionVPNOff: {"🌐", "выключить VPN", "🌐 Выключить VPN"},
	domain.ActionVPNOn:  {"🔒", "включить VPN", "🔒 Включить VPN"},
}

// ActionLabel — человекочитаемое действие для строк и подсказок.
func ActionLabel(a domain.Action) string {
	if i, ok := actionInfo[a]; ok {
		return i.label
	}
	return "?"
}

// ActionButton — подпись кнопки выбора действия (plain text: HTML в кнопках
// Telegram не рендерится).
func ActionButton(a domain.Action) string {
	if i, ok := actionInfo[a]; ok {
		return i.button
	}
	return "⏰ ?"
}

// actionEmoji — иконка действия для строк активных таймеров.
func actionEmoji(a domain.Action) string {
	if i, ok := actionInfo[a]; ok {
		return i.emoji
	}
	return "⏰"
}

// RemainingText — остаток до срабатывания словами: «через 42 мин» / «через 1 ч
// 05 мин». Меньше минуты (вот-вот сработает) → «вот-вот».
func RemainingText(d time.Duration) string {
	if d < time.Minute {
		return "вот-вот"
	}
	total := int(d / time.Minute)
	h, m := total/60, total%60
	if h == 0 {
		return fmt.Sprintf("через %d мин", m)
	}
	return fmt.Sprintf("через %d ч %02d мин", h, m)
}

// MinutesLabel — подпись кнопки-пресета задержки: «30 мин» / «1 ч» / «1 ч 30 мин».
func MinutesLabel(mins int) string {
	if mins < 60 {
		return fmt.Sprintf("%d мин", mins)
	}
	if mins%60 == 0 {
		return fmt.Sprintf("%d ч", mins/60)
	}
	return fmt.Sprintf("%d ч %02d мин", mins/60, mins%60)
}

// Timers — корневой экран (HTML): заголовок + активные таймеры. names — заранее
// собранные вызывающим имена по MAC (устройство могло уйти из DHCP — тогда
// показываем MAC).
func Timers(tasks []timer.View, names map[domain.MAC]string) string {
	if len(tasks) == 0 {
		return "<b>⏰ Таймеры</b>\n\nАктивных нет.\nВыбери устройство, чтобы поставить."
	}
	var b strings.Builder
	b.WriteString("<b>⏰ Таймеры</b>\n\nАктивные:\n")
	for _, v := range tasks {
		fmt.Fprintf(&b, "%s <b>%s</b> — %s, %s\n",
			actionEmoji(v.Job.Action), html.EscapeString(DeviceNameOr(names, v.Job.MAC)),
			ActionLabel(v.Job.Action), RemainingText(v.Remaining))
	}
	b.WriteString("\nВыбери устройство, чтобы поставить ещё:")
	return b.String()
}

// TimerActionPrompt — экран выбора действия для выбранного устройства.
func TimerActionPrompt(deviceName string) string {
	return fmt.Sprintf("<b>⏰ Таймер · %s</b>\n\nЧто запланировать?", html.EscapeString(deviceName))
}

// TimerMinutesPrompt — экран выбора задержки: устройство + уже выбранное действие.
func TimerMinutesPrompt(deviceName string, a domain.Action) string {
	return fmt.Sprintf("<b>⏰ Таймер · %s</b>\nДействие: %s %s\n\nЧерез сколько?",
		html.EscapeString(deviceName), actionEmoji(a), ActionLabel(a))
}

// Scheduled — подтверждение постановки таймера (для текст-команды /timer).
func Scheduled(mac domain.MAC, a domain.Action, mins domain.Minutes) string {
	return fmt.Sprintf("⏰ таймер на <code>%s</code>: %s %s",
		mac, ActionLabel(a), RemainingText(mins.Duration()))
}

// DeviceNameOr — имя устройства по MAC или сам MAC, если устройства нет в
// карте (ушло из DHCP-лиз). Общая точка для текста экрана и подписей кнопок.
func DeviceNameOr(names map[domain.MAC]string, mac domain.MAC) string {
	if name, ok := names[mac]; ok {
		return name
	}
	return mac.String()
}

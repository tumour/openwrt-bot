// Package presenter форматирует use case Output → Telegram message text.
// Это та часть, которая знает специфику канала (эмодзи, длина, разметка).
// Domain и app ничего этого не знают.
//
// Вся разметка — ТОЛЬКО HTML (tele.ModeHTML): в отличие от Markdown V1 у него
// типизированное экранирование (html.EscapeString) для внешних строк — имён
// хостов, серверов и прочего, что бот не контролирует. Не смешивать режимы.
package presenter

import (
	"fmt"
	"strings"
	"time"

	"github.com/tumour/openwrt-bot/internal/app/status"
)

// Status форматирует Snapshot в текстовое сообщение (HTML).
func Status(s status.Snapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<b>Uptime:</b> %s\n", uptime(s.Uptime))
	fmt.Fprintf(&b, "<b>Load:</b> %.2f / %.2f / %.2f\n", s.LoadAvg1, s.LoadAvg5, s.LoadAvg15)
	fmt.Fprintf(&b, "<b>Memory:</b> %s свободно из %s\n", kb(s.MemFreeKB), kb(s.MemTotalKB))
	if s.TempCelsius > 0 {
		fmt.Fprintf(&b, "<b>Temp:</b> %.1f°C\n", s.TempCelsius)
	}
	return b.String()
}

func uptime(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	if days > 0 {
		return fmt.Sprintf("%dd %02dh %02dm", days, hours, minutes)
	}
	return fmt.Sprintf("%02dh %02dm", hours, minutes)
}

func kb(v uint64) string {
	switch {
	case v < 1024:
		return fmt.Sprintf("%d KB", v)
	case v < 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(v)/1024)
	default:
		return fmt.Sprintf("%.2f GB", float64(v)/1024/1024)
	}
}

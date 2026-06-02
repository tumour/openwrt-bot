package presenter

import (
	"fmt"
	"html"
	"strings"

	"github.com/tumour/openwrt-bot/internal/app/device"
)

// DeviceList форматирует список устройств для Telegram (режим HTML).
// Раскладка вертикальная, а не моноширинная таблица: таблица переносит колонки
// на узких экранах и выглядит криво. MAC каждого устройства — отдельный
// <code>-спан, поэтому тап по нему копирует именно этот MAC, а не весь список.
func DeviceList(views []device.View) string {
	if len(views) == 0 {
		return "<i>LAN пуст</i>"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "<b>Устройства в LAN (%d):</b>\n", len(views))
	for _, v := range views {
		icon := "📱"
		if v.Banned {
			icon = "🚫"
		}
		host := v.Device.Hostname
		if host == "" {
			host = "без имени"
		}
		ip := "—"
		if v.Device.IP != nil {
			ip = v.Device.IP.String()
		}
		// Имя/IP экранируем (вдруг в hostname есть <>&), MAC — для единообразия.
		fmt.Fprintf(&b, "\n%s %s · %s\n<code>%s</code>\n",
			icon, html.EscapeString(host), html.EscapeString(ip), html.EscapeString(v.Device.MAC.String()))
	}
	return b.String()
}

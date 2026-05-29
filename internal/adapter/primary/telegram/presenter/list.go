package presenter

import (
	"fmt"
	"strings"

	"github.com/tumour/openwrt-bot/internal/app/device"
)

// DeviceList форматирует список устройств в моноширинную таблицу с пометкой бана.
// Telegram Markdown-блок `code` сохраняет выравнивание.
func DeviceList(views []device.View) string {
	if len(views) == 0 {
		return "_LAN пуст_"
	}

	var b strings.Builder
	b.WriteString("*Устройства в LAN:*\n```\n")
	for _, v := range views {
		flag := "  "
		if v.Banned {
			flag = "🔴"
		}
		ip := "-"
		if v.Device.IP != nil {
			ip = v.Device.IP.String()
		}
		host := v.Device.Hostname
		if host == "" {
			host = "-"
		}
		b.WriteString(fmt.Sprintf("%s %-17s %-15s %s\n",
			flag, v.Device.MAC, ip, host))
	}
	b.WriteString("```")
	return b.String()
}

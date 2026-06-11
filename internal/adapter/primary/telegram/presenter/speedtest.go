package presenter

import (
	"fmt"
	"html"
	"strings"

	"github.com/tumour/openwrt-bot/internal/app/network"
)

// SpeedTest форматирует результат замера (HTML). Имя сервера — внешние данные
// из librespeed и экранируется обязательно: раньше «_» или «*» в имени ломали
// Markdown-разметку, Telegram отвечал 400 «can't parse entities», Edit падал —
// и юзер навсегда оставался с «⏳ замеряю…».
func SpeedTest(r network.SpeedResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "⬇ <b>Download:</b> %.1f Mbps\n", r.DownloadMbps)
	fmt.Fprintf(&b, "⬆ <b>Upload:</b> %.1f Mbps\n", r.UploadMbps)
	fmt.Fprintf(&b, "📶 <b>Ping:</b> %.0f ms (jitter %.1f)\n", r.PingMs, r.JitterMs)
	if r.Server != "" {
		fmt.Fprintf(&b, "<i>сервер: %s</i>", html.EscapeString(r.Server))
	}
	return b.String()
}

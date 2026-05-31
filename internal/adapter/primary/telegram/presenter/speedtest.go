package presenter

import (
	"fmt"
	"strings"

	"github.com/tumour/openwrt-bot/internal/app/network"
)

// SpeedTest форматирует результат замера в Markdown-сообщение.
func SpeedTest(r network.SpeedResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "⬇ *Download:* %.1f Mbps\n", r.DownloadMbps)
	fmt.Fprintf(&b, "⬆ *Upload:* %.1f Mbps\n", r.UploadMbps)
	fmt.Fprintf(&b, "📶 *Ping:* %.0f ms (jitter %.1f)\n", r.PingMs, r.JitterMs)
	if r.Server != "" {
		fmt.Fprintf(&b, "_сервер: %s_", r.Server)
	}
	return b.String()
}

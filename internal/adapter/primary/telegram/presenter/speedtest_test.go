package presenter

import (
	"strings"
	"testing"

	"github.com/tumour/openwrt-bot/internal/app/network"
)

// Имя сервера приходит из librespeed как есть — спецсимволы обязаны
// экранироваться, иначе Telegram отвергнет сообщение и Edit упадёт
// (а юзер останется с вечным «⏳ замеряю…»).
func TestSpeedTest_EscapesServerName(t *testing.T) {
	got := SpeedTest(network.SpeedResult{Server: `Cool <ISP> & "Friends"`})
	if strings.Contains(got, "<ISP>") {
		t.Errorf("имя сервера не экранировано: %q", got)
	}
	if !strings.Contains(got, "&lt;ISP&gt;") {
		t.Errorf("ожидалось html-экранирование имени сервера: %q", got)
	}
}

func TestSpeedTest_Format(t *testing.T) {
	got := SpeedTest(network.SpeedResult{
		DownloadMbps: 93.4, UploadMbps: 11.2,
		PingMs: 14, JitterMs: 1.5,
		Server: "Volzhsky, Russia (PowerNet)",
	})
	for _, want := range []string{"93.4", "11.2", "14 ms", "jitter 1.5", "сервер: Volzhsky"} {
		if !strings.Contains(got, want) {
			t.Errorf("в выводе нет %q: %q", want, got)
		}
	}
}

// Пустое имя сервера (авто-выбор не вернул name) — строка сервера скрывается.
func TestSpeedTest_NoServerLine(t *testing.T) {
	if got := SpeedTest(network.SpeedResult{DownloadMbps: 1}); strings.Contains(got, "сервер") {
		t.Errorf("строка сервера не должна появляться без имени: %q", got)
	}
}

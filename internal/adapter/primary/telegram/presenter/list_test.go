package presenter

import (
	"net"
	"strings"
	"testing"

	"github.com/tumour/openwrt-bot/internal/app/device"
	"github.com/tumour/openwrt-bot/internal/domain"
)

func mustMAC(t *testing.T, s string) domain.MAC {
	t.Helper()
	m, err := domain.NewMAC(s)
	if err != nil {
		t.Fatalf("NewMAC(%q): %v", s, err)
	}
	return m
}

func view(t *testing.T, host, ip string, banned, direct bool) device.View {
	t.Helper()
	return device.View{
		Device: domain.Device{MAC: mustMAC(t, "aa:bb:cc:dd:ee:ff"), Hostname: host, IP: net.ParseIP(ip)},
		Banned: banned,
		Direct: direct,
	}
}

func TestListHeader(t *testing.T) {
	if got := ListHeader(0); got != "<i>LAN пуст</i>" {
		t.Errorf("пустой список = %q", got)
	}
	if got := ListHeader(3); !strings.Contains(got, "(3)") {
		t.Errorf("нет счётчика устройств: %q", got)
	}
}

func TestDeviceLabel(t *testing.T) {
	// Plain text без HTML — Telegram не рендерит теги в кнопках.
	got := DeviceLabel(view(t, "iPhone", "192.168.1.10", false, false))
	if got != "📱 iPhone · 192.168.1.10" {
		t.Errorf("label = %q", got)
	}
	if got := DeviceLabel(view(t, "", "192.168.1.11", true, false)); !strings.Contains(got, "🚫 без имени") {
		t.Errorf("бан/пустое имя: %q", got)
	}
	if got := DeviceLabel(view(t, "pc", "192.168.1.12", false, true)); !strings.HasPrefix(got, "🌐") {
		t.Errorf("vpn-обход не помечен 🌐: %q", got)
	}
	// Бан перекрывает обход.
	if got := DeviceLabel(view(t, "pc", "192.168.1.12", true, true)); !strings.HasPrefix(got, "🚫") {
		t.Errorf("бан должен перекрывать обход: %q", got)
	}
}

func TestDeviceCard(t *testing.T) {
	got := DeviceCard(view(t, "<script>", "192.168.1.10", false, true))

	// MAC — отдельный копируемый <code>-спан (главное требование).
	if !strings.Contains(got, "<code>aa:bb:cc:dd:ee:ff</code>") {
		t.Error("MAC не обёрнут в <code>")
	}
	if !strings.Contains(got, "Бан: нет") || !strings.Contains(got, "напрямую") {
		t.Errorf("статусы неверные: %q", got)
	}
	// HTML-экранирование: hostname с <> не должен утечь как сырой тег.
	if strings.Contains(got, "<script>") || !strings.Contains(got, "&lt;script&gt;") {
		t.Error("hostname с <> не экранирован")
	}
}

func TestDeviceCard_Banned(t *testing.T) {
	got := DeviceCard(view(t, "tv", "192.168.1.20", true, false))
	if !strings.Contains(got, "🚫 забанен") || !strings.Contains(got, "через VPN") {
		t.Errorf("статусы карточки: %q", got)
	}
}

func TestCardGone_EscapesMAC(t *testing.T) {
	got := CardGone("<x>")
	if strings.Contains(got, "<x>") {
		t.Errorf("MAC не экранирован: %q", got)
	}
}

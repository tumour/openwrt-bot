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

func TestDeviceList_Empty(t *testing.T) {
	if got := DeviceList(nil); got != "<i>LAN пуст</i>" {
		t.Errorf("пустой список = %q", got)
	}
}

func TestDeviceList_Format(t *testing.T) {
	views := []device.View{
		{Device: domain.Device{MAC: mustMAC(t, "aa:bb:cc:dd:ee:ff"), Hostname: "iPhone", IP: net.ParseIP("192.168.1.10")}},
		{Device: domain.Device{MAC: mustMAC(t, "11:22:33:44:55:66"), Hostname: "", IP: net.ParseIP("192.168.1.11")}, Banned: true},
		{Device: domain.Device{MAC: mustMAC(t, "de:ad:be:ef:00:01"), Hostname: "<script>", IP: nil}},
	}
	out := DeviceList(views)

	// MAC — отдельный копируемый <code>-спан (главное требование).
	if !strings.Contains(out, "<code>aa:bb:cc:dd:ee:ff</code>") {
		t.Error("MAC не обёрнут в <code>")
	}
	if !strings.Contains(out, "Устройства в LAN (3):") {
		t.Error("нет заголовка со счётчиком")
	}
	if !strings.Contains(out, "🚫") {
		t.Error("забаненное устройство не помечено 🚫")
	}
	if !strings.Contains(out, "без имени") {
		t.Error("пустой hostname не заменён на «без имени»")
	}
	if !strings.Contains(out, "—") {
		t.Error("nil IP не заменён на —")
	}
	// HTML-экранирование: hostname с <> не должен утечь как сырой тег.
	if !strings.Contains(out, "&lt;script&gt;") {
		t.Error("hostname с <> не экранирован")
	}
	if strings.Contains(out, "<script>") {
		t.Error("в выводе сырой неэкранированный <script>")
	}
}

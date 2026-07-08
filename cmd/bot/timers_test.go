package main

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/tumour/openwrt-bot/internal/app/device"
	"github.com/tumour/openwrt-bot/internal/domain"
)

// stubSet — MACSetPort, фиксирующий Add/Remove. Один и тот же порт-паттерн, что
// и у боевого nftables-адаптера, поэтому use cases ведут себя как в проде.
type stubSet struct {
	added   []domain.MAC
	removed []domain.MAC
}

func (s *stubSet) Add(_ context.Context, mac domain.MAC) error {
	s.added = append(s.added, mac)
	return nil
}
func (s *stubSet) Remove(_ context.Context, mac domain.MAC) error {
	s.removed = append(s.removed, mac)
	return nil
}
func (s *stubSet) List(_ context.Context) ([]domain.MAC, error) { return nil, nil }

// deviceRunner обязан дёргать РОВНО те же use cases (и, значит, сеты), что и
// кнопки карточки: ban/unban → banned-сет, vpnoff/vpnon → direct-сет. Иначе
// таймер и ручное действие разойдутся — этого мы и боялись.
func TestDeviceRunner_DispatchesActionToRightSet(t *testing.T) {
	banned, direct := &stubSet{}, &stubSet{}
	r := deviceRunner{
		ban:    device.NewBan(banned),
		unban:  device.NewUnban(banned),
		vpnOff: device.NewDisableVPN(direct),
		vpnOn:  device.NewEnableVPN(direct),
		log:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	mac := mustMAC(t, "aa:bb:cc:11:22:33")

	cases := []struct {
		action domain.Action
		ok     func() bool
		desc   string
	}{
		{domain.ActionBan, func() bool { return len(banned.added) == 1 && banned.added[0] == mac }, "ban → banned.Add"},
		{domain.ActionUnban, func() bool { return len(banned.removed) == 1 && banned.removed[0] == mac }, "unban → banned.Remove"},
		{domain.ActionVPNOff, func() bool { return len(direct.added) == 1 && direct.added[0] == mac }, "vpnoff → direct.Add"},
		{domain.ActionVPNOn, func() bool { return len(direct.removed) == 1 && direct.removed[0] == mac }, "vpnon → direct.Remove"},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			if err := r.Run(context.Background(), domain.DeviceJob{MAC: mac, Action: tc.action}); err != nil {
				t.Fatalf("Run: %v", err)
			}
			if !tc.ok() {
				t.Errorf("%s: действие ушло не туда", tc.desc)
			}
		})
	}
}

func TestDeviceRunner_UnknownAction(t *testing.T) {
	r := deviceRunner{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if err := r.Run(context.Background(), domain.DeviceJob{Action: domain.Action(99)}); err == nil {
		t.Fatal("ждём ошибку на неизвестном действии")
	}
}

func mustMAC(t *testing.T, s string) domain.MAC {
	t.Helper()
	mac, err := domain.NewMAC(s)
	if err != nil {
		t.Fatalf("NewMAC(%q): %v", s, err)
	}
	return mac
}

package device

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// stubDhcpPort — fake реализация DhcpPort. Возвращает заранее заданный список устройств.
type stubDhcpPort struct {
	leases []domain.Device
	err    error
}

func (s *stubDhcpPort) ListLeases(_ context.Context) ([]domain.Device, error) {
	return s.leases, s.err
}

// stubMACSet — fake реализация MACSetPort (для теста List нужен только List).
// Реализуем все три метода, чтобы тип удовлетворял интерфейсу.
type stubMACSet struct {
	macs []domain.MAC
	err  error
}

func (s *stubMACSet) Add(_ context.Context, _ domain.MAC) error    { return nil }
func (s *stubMACSet) Remove(_ context.Context, _ domain.MAC) error { return nil }
func (s *stubMACSet) List(_ context.Context) ([]domain.MAC, error) {
	return s.macs, s.err
}

// stubRateLimits — fake реализация RateLimitPort (для теста List нужен только List).
type stubRateLimits struct {
	limits map[domain.MAC]domain.Rate
	err    error
}

func (s *stubRateLimits) Set(_ context.Context, _ domain.MAC, _ domain.Rate) error { return nil }
func (s *stubRateLimits) Remove(_ context.Context, _ domain.MAC) error             { return nil }
func (s *stubRateLimits) List(_ context.Context) (map[domain.MAC]domain.Rate, error) {
	return s.limits, s.err
}

func newMAC(t *testing.T, s string) domain.MAC {
	t.Helper()
	m, err := domain.NewMAC(s)
	if err != nil {
		t.Fatalf("newMAC(%q): %v", s, err)
	}
	return m
}

func TestList_Execute_OrchestratesTwoPorts(t *testing.T) {
	a := newMAC(t, "aa:bb:cc:11:22:33")
	b := newMAC(t, "11:22:33:44:55:66")
	c := newMAC(t, "ff:ee:dd:cc:bb:aa")

	dhcp := &stubDhcpPort{leases: []domain.Device{
		{MAC: a, Hostname: "laptop", IP: net.ParseIP("192.168.88.42")},
		{MAC: b, Hostname: "phone", IP: net.ParseIP("192.168.88.43")},
		{MAC: c, Hostname: "tv", IP: net.ParseIP("192.168.88.44")},
	}}
	banned := &stubMACSet{macs: []domain.MAC{b}} // забанен только phone
	direct := &stubMACSet{macs: []domain.MAC{c}} // tv ходит мимо VPN
	rate, _ := domain.NewRate(512)
	limits := &stubRateLimits{limits: map[domain.MAC]domain.Rate{a: rate}} // лимит у laptop

	uc := NewList(dhcp, banned, direct, limits)
	out, err := uc.Execute(context.Background(), ListInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Devices) != 3 {
		t.Fatalf("got %d views, want 3", len(out.Devices))
	}

	// Проверяем правильные пометки Banned/Direct/Limit по MAC, независимо от порядка.
	byMAC := map[domain.MAC]View{}
	for _, v := range out.Devices {
		byMAC[v.Device.MAC] = v
	}
	if byMAC[a].Banned || !byMAC[b].Banned || byMAC[c].Banned {
		t.Errorf("banned flags wrong: %+v", byMAC)
	}
	if byMAC[a].Direct || byMAC[b].Direct || !byMAC[c].Direct {
		t.Errorf("direct flags wrong: %+v", byMAC)
	}
	if byMAC[a].Limit != rate || !byMAC[b].Limit.IsZero() || !byMAC[c].Limit.IsZero() {
		t.Errorf("limit flags wrong: %+v", byMAC)
	}
}

// Лимиты в /list — best-effort: ошибка RateLimitPort (например, старый init.d
// без netdev-таблицы) не роняет список, устройства приходят без пометки лимита.
func TestList_Execute_RateLimitError_BestEffort(t *testing.T) {
	a := newMAC(t, "aa:bb:cc:11:22:33")
	dhcp := &stubDhcpPort{leases: []domain.Device{
		{MAC: a, Hostname: "laptop", IP: net.ParseIP("192.168.88.42")},
	}}
	limits := &stubRateLimits{err: errors.New("no such table")}

	uc := NewList(dhcp, &stubMACSet{}, &stubMACSet{}, limits)
	out, err := uc.Execute(context.Background(), ListInput{})
	if err != nil {
		t.Fatalf("rate limit error must not fail /list: %v", err)
	}
	if len(out.Devices) != 1 || !out.Devices[0].Limit.IsZero() {
		t.Errorf("expected device without limit mark, got %+v", out.Devices)
	}
}

func TestList_Execute_DhcpError(t *testing.T) {
	dhcpErr := errors.New("leases file vanished")
	uc := NewList(&stubDhcpPort{err: dhcpErr}, &stubMACSet{}, &stubMACSet{}, &stubRateLimits{})

	_, err := uc.Execute(context.Background(), ListInput{})
	if err == nil || !errors.Is(err, dhcpErr) {
		t.Errorf("expected wrapped dhcp error, got: %v", err)
	}
}

func TestList_Execute_NftError(t *testing.T) {
	nftErr := errors.New("nft list failed")
	uc := NewList(&stubDhcpPort{}, &stubMACSet{err: nftErr}, &stubMACSet{}, &stubRateLimits{})

	_, err := uc.Execute(context.Background(), ListInput{})
	if err == nil || !errors.Is(err, nftErr) {
		t.Errorf("expected wrapped nft error, got: %v", err)
	}
}

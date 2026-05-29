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

// stubNftList — fake реализация NftPort (для теста List нужны только AddBanned/RemoveBanned
// заглушки + рабочий ListBanned). Реализуем все три метода, чтобы тип удовлетворял интерфейсу.
type stubNftList struct {
	banned []domain.MAC
	err    error
}

func (s *stubNftList) AddBanned(_ context.Context, _ domain.MAC) error    { return nil }
func (s *stubNftList) RemoveBanned(_ context.Context, _ domain.MAC) error { return nil }
func (s *stubNftList) ListBanned(_ context.Context) ([]domain.MAC, error) {
	return s.banned, s.err
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
	nft := &stubNftList{banned: []domain.MAC{b}} // забанен только phone

	uc := NewList(dhcp, nft)
	out, err := uc.Execute(context.Background(), ListInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Devices) != 3 {
		t.Fatalf("got %d views, want 3", len(out.Devices))
	}

	// Проверяем правильную пометку Banned по MAC, независимо от порядка.
	bannedByMAC := map[domain.MAC]bool{}
	for _, v := range out.Devices {
		bannedByMAC[v.Device.MAC] = v.Banned
	}
	if bannedByMAC[a] || !bannedByMAC[b] || bannedByMAC[c] {
		t.Errorf("banned flags wrong: %v", bannedByMAC)
	}
}

func TestList_Execute_DhcpError(t *testing.T) {
	dhcpErr := errors.New("leases file vanished")
	uc := NewList(&stubDhcpPort{err: dhcpErr}, &stubNftList{})

	_, err := uc.Execute(context.Background(), ListInput{})
	if err == nil || !errors.Is(err, dhcpErr) {
		t.Errorf("expected wrapped dhcp error, got: %v", err)
	}
}

func TestList_Execute_NftError(t *testing.T) {
	nftErr := errors.New("nft list failed")
	uc := NewList(&stubDhcpPort{}, &stubNftList{err: nftErr})

	_, err := uc.Execute(context.Background(), ListInput{})
	if err == nil || !errors.Is(err, nftErr) {
		t.Errorf("expected wrapped nft error, got: %v", err)
	}
}

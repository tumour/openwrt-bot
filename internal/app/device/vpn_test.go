package device

import (
	"context"
	"errors"
	"testing"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// stubDirectSet — стаб MACSetPort для vpn-тестов: пишет вызовы Add/Remove.
type stubDirectSet struct {
	addErr    error
	removeErr error
	gotAdd    domain.MAC
	gotRemove domain.MAC
}

func (s *stubDirectSet) Add(_ context.Context, mac domain.MAC) error {
	s.gotAdd = mac
	return s.addErr
}
func (s *stubDirectSet) Remove(_ context.Context, mac domain.MAC) error {
	s.gotRemove = mac
	return s.removeErr
}
func (s *stubDirectSet) List(_ context.Context) ([]domain.MAC, error) { return nil, nil }

func TestDisableVPN_Execute_OK(t *testing.T) {
	port := &stubDirectSet{}
	uc := NewDisableVPN(port)
	mac := newMAC(t, "aa:bb:cc:11:22:33")

	if _, err := uc.Execute(context.Background(), DisableVPNInput{MAC: mac}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port.gotAdd != mac {
		t.Errorf("Add called with %s, want %s", port.gotAdd, mac)
	}
}

func TestDisableVPN_Execute_AlreadyDirect_IsNoOp(t *testing.T) {
	// Повторный /vpnoff = no-op, как повторный бан.
	port := &stubDirectSet{addErr: domain.ErrAlreadyInSet}
	uc := NewDisableVPN(port)

	if _, err := uc.Execute(context.Background(), DisableVPNInput{MAC: newMAC(t, "aa:bb:cc:11:22:33")}); err != nil {
		t.Errorf("ErrAlreadyInSet should be swallowed; got: %v", err)
	}
}

func TestDisableVPN_Execute_OtherError_Propagates(t *testing.T) {
	other := errors.New("nft: io error")
	uc := NewDisableVPN(&stubDirectSet{addErr: other})

	_, err := uc.Execute(context.Background(), DisableVPNInput{MAC: newMAC(t, "aa:bb:cc:11:22:33")})
	if err == nil || !errors.Is(err, other) {
		t.Errorf("expected wrapped %v; got: %v", other, err)
	}
}

func TestEnableVPN_Execute_OK(t *testing.T) {
	port := &stubDirectSet{}
	uc := NewEnableVPN(port)
	mac := newMAC(t, "aa:bb:cc:11:22:33")

	if _, err := uc.Execute(context.Background(), EnableVPNInput{MAC: mac}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port.gotRemove != mac {
		t.Errorf("Remove called with %s, want %s", port.gotRemove, mac)
	}
}

func TestEnableVPN_Execute_NotDirect_IsNoOp(t *testing.T) {
	// /vpnon для устройства, которое и так в VPN — no-op.
	port := &stubDirectSet{removeErr: domain.ErrNotInSet}
	uc := NewEnableVPN(port)

	if _, err := uc.Execute(context.Background(), EnableVPNInput{MAC: newMAC(t, "aa:bb:cc:11:22:33")}); err != nil {
		t.Errorf("ErrNotInSet should be swallowed; got: %v", err)
	}
}

func TestEnableVPN_Execute_OtherError_Propagates(t *testing.T) {
	other := errors.New("nft: io error")
	uc := NewEnableVPN(&stubDirectSet{removeErr: other})

	_, err := uc.Execute(context.Background(), EnableVPNInput{MAC: newMAC(t, "aa:bb:cc:11:22:33")})
	if err == nil || !errors.Is(err, other) {
		t.Errorf("expected wrapped %v; got: %v", other, err)
	}
}

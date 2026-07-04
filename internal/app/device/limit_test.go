package device

import (
	"context"
	"errors"
	"testing"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// stubRatePort — ручной стаб RateLimitPort (по образцу stubNftPort).
type stubRatePort struct {
	setErr     error
	setCalled  int
	gotSetMAC  domain.MAC
	gotSetRate domain.Rate

	removeErr    error
	removeCalled int
	gotRemoveMAC domain.MAC
}

func (s *stubRatePort) Set(_ context.Context, mac domain.MAC, rate domain.Rate) error {
	s.setCalled++
	s.gotSetMAC = mac
	s.gotSetRate = rate
	return s.setErr
}

func (s *stubRatePort) Remove(_ context.Context, mac domain.MAC) error {
	s.removeCalled++
	s.gotRemoveMAC = mac
	return s.removeErr
}

func (s *stubRatePort) List(_ context.Context) (map[domain.MAC]domain.Rate, error) {
	return nil, nil
}

func TestSetLimit_Execute_OK(t *testing.T) {
	port := &stubRatePort{}
	uc := NewSetLimit(port)
	mac, _ := domain.NewMAC("aa:bb:cc:11:22:33")
	rate, _ := domain.NewRate(512)

	_, err := uc.Execute(context.Background(), SetLimitInput{MAC: mac, Rate: rate})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port.setCalled != 1 || port.gotSetMAC != mac || port.gotSetRate != rate {
		t.Errorf("port called with %d / %s / %s", port.setCalled, port.gotSetMAC, port.gotSetRate)
	}
}

func TestSetLimit_Execute_Error_Propagates(t *testing.T) {
	other := errors.New("nft: permission denied")
	port := &stubRatePort{setErr: other}
	uc := NewSetLimit(port)
	mac, _ := domain.NewMAC("aa:bb:cc:11:22:33")
	rate, _ := domain.NewRate(512)

	_, err := uc.Execute(context.Background(), SetLimitInput{MAC: mac, Rate: rate})
	if err == nil || !errors.Is(err, other) {
		t.Errorf("expected wrapped %v; got: %v", other, err)
	}
}

func TestRemoveLimit_Execute_OK(t *testing.T) {
	port := &stubRatePort{}
	uc := NewRemoveLimit(port)
	mac, _ := domain.NewMAC("aa:bb:cc:11:22:33")

	_, err := uc.Execute(context.Background(), RemoveLimitInput{MAC: mac})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port.removeCalled != 1 || port.gotRemoveMAC != mac {
		t.Errorf("port called with %d / %s", port.removeCalled, port.gotRemoveMAC)
	}
}

func TestRemoveLimit_Execute_Error_Propagates(t *testing.T) {
	other := errors.New("nft: permission denied")
	port := &stubRatePort{removeErr: other}
	uc := NewRemoveLimit(port)
	mac, _ := domain.NewMAC("aa:bb:cc:11:22:33")

	_, err := uc.Execute(context.Background(), RemoveLimitInput{MAC: mac})
	if err == nil || !errors.Is(err, other) {
		t.Errorf("expected wrapped %v; got: %v", other, err)
	}
}

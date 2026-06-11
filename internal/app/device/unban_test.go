package device

import (
	"context"
	"errors"
	"testing"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// stubNftRemove — отдельный stub под Unban-тест, чтобы не мешать Ban-тестам
// общим счётчиком. Намеренно мелкий — реализуем только то, что нужно use case.
type stubNftRemove struct {
	removeErr    error
	removeCalled int
	gotRemoveMAC domain.MAC
}

func (s *stubNftRemove) AddBanned(_ context.Context, _ domain.MAC) error { return nil }
func (s *stubNftRemove) RemoveBanned(_ context.Context, mac domain.MAC) error {
	s.removeCalled++
	s.gotRemoveMAC = mac
	return s.removeErr
}
func (s *stubNftRemove) ListBanned(_ context.Context) ([]domain.MAC, error) { return nil, nil }

func TestUnban_Execute_OK(t *testing.T) {
	port := &stubNftRemove{}
	uc := NewUnban(port)
	mac, _ := domain.NewMAC("aa:bb:cc:11:22:33")

	_, err := uc.Execute(context.Background(), UnbanInput{MAC: mac})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port.removeCalled != 1 || port.gotRemoveMAC != mac {
		t.Errorf("port called with %d / %s", port.removeCalled, port.gotRemoveMAC)
	}
}

func TestUnban_Execute_NotBanned_IsNoOp(t *testing.T) {
	port := &stubNftRemove{removeErr: domain.ErrNotBanned}
	uc := NewUnban(port)
	mac, _ := domain.NewMAC("aa:bb:cc:11:22:33")

	_, err := uc.Execute(context.Background(), UnbanInput{MAC: mac})
	if err != nil {
		t.Errorf("ErrNotBanned should be swallowed; got: %v", err)
	}
}

func TestUnban_Execute_OtherError_Propagates(t *testing.T) {
	other := errors.New("nft: io error")
	port := &stubNftRemove{removeErr: other}
	uc := NewUnban(port)
	mac, _ := domain.NewMAC("aa:bb:cc:11:22:33")

	_, err := uc.Execute(context.Background(), UnbanInput{MAC: mac})
	if err == nil || !errors.Is(err, other) {
		t.Errorf("expected wrapped %v; got: %v", other, err)
	}
}

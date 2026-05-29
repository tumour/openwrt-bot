package device

import (
	"context"
	"errors"
	"testing"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// stubNftPort реализует только методы, нужные для Ban-теста. Это легитимно —
// порты сделаны узкими специально, чтобы стабы оставались мелкими.
type stubNftPort struct {
	addErr     error
	addCalled  int
	gotAddMAC  domain.MAC
}

func (s *stubNftPort) AddBanned(_ context.Context, mac domain.MAC) error {
	s.addCalled++
	s.gotAddMAC = mac
	return s.addErr
}
func (s *stubNftPort) RemoveBanned(_ context.Context, _ domain.MAC) error { return nil }
func (s *stubNftPort) ListBanned(_ context.Context) ([]domain.MAC, error) { return nil, nil }

func TestBan_Execute_OK(t *testing.T) {
	port := &stubNftPort{}
	uc := NewBan(port)
	mac, _ := domain.NewMAC("aa:bb:cc:11:22:33")

	_, err := uc.Execute(context.Background(), BanInput{MAC: mac})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port.addCalled != 1 || port.gotAddMAC != mac {
		t.Errorf("port called with %d / %s", port.addCalled, port.gotAddMAC)
	}
}

func TestBan_Execute_AlreadyBanned_IsNoOp(t *testing.T) {
	// Application rule: повторный бан = no-op для вызывающего. Use case
	// должен "проглотить" доменную ошибку ErrAlreadyBanned и вернуть nil.
	port := &stubNftPort{addErr: domain.ErrAlreadyBanned}
	uc := NewBan(port)
	mac, _ := domain.NewMAC("aa:bb:cc:11:22:33")

	_, err := uc.Execute(context.Background(), BanInput{MAC: mac})
	if err != nil {
		t.Errorf("ErrAlreadyBanned should be swallowed; got: %v", err)
	}
}

func TestBan_Execute_OtherError_Propagates(t *testing.T) {
	other := errors.New("nft: permission denied")
	port := &stubNftPort{addErr: other}
	uc := NewBan(port)
	mac, _ := domain.NewMAC("aa:bb:cc:11:22:33")

	_, err := uc.Execute(context.Background(), BanInput{MAC: mac})
	if err == nil || !errors.Is(err, other) {
		t.Errorf("expected wrapped %v; got: %v", other, err)
	}
}

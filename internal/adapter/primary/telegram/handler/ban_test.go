package handler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/tumour/openwrt-bot/internal/app/device"
	"github.com/tumour/openwrt-bot/internal/domain"
	tele "gopkg.in/telebot.v3"
)

// fakeCtx — минимальный мок tele.Context (embed нилового интерфейса, см.
// middleware/auth_test.go). Записывает отправленные сообщения.
type fakeCtx struct {
	tele.Context
	args []string
	sent []string
}

func (f *fakeCtx) Args() []string { return f.args }
func (f *fakeCtx) Send(what interface{}, _ ...interface{}) error {
	f.sent = append(f.sent, fmt.Sprint(what))
	return nil
}

// stubNft — стаб device.NftPort: Ban-хендлеру нужен только AddBanned.
type stubNft struct{ addErr error }

func (s *stubNft) AddBanned(context.Context, domain.MAC) error    { return s.addErr }
func (s *stubNft) RemoveBanned(context.Context, domain.MAC) error { return nil }
func (s *stubNft) ListBanned(context.Context) ([]domain.MAC, error) {
	return nil, nil
}

func newBanHandler(addErr error) *Ban {
	return NewBan(device.NewBan(&stubNft{addErr: addErr}))
}

func TestBan_NoArgs_ShowsUsage(t *testing.T) {
	c := &fakeCtx{}
	if err := newBanHandler(nil).Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(c.sent) != 1 || !strings.Contains(c.sent[0], "использование") {
		t.Errorf("ожидалась подсказка по использованию, got: %v", c.sent)
	}
}

func TestBan_InvalidMAC_ShowsWarning(t *testing.T) {
	c := &fakeCtx{args: []string{"not-a-mac"}}
	if err := newBanHandler(nil).Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(c.sent) != 1 || !strings.Contains(c.sent[0], "невалидный MAC") {
		t.Errorf("ожидалось предупреждение о MAC, got: %v", c.sent)
	}
}

func TestBan_OK_SendsConfirmation(t *testing.T) {
	c := &fakeCtx{args: []string{"AA:BB:CC:11:22:33"}}
	if err := newBanHandler(nil).Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(c.sent) != 1 || !strings.Contains(c.sent[0], "aa:bb:cc:11:22:33") {
		t.Errorf("ожидалось подтверждение с нормализованным MAC, got: %v", c.sent)
	}
}

// Контракт ошибок: юзеру — короткое сообщение без внутренностей exec,
// полная цепочка уходит наверх (её логирует middleware.Log).
func TestBan_UCError_ShortMessageFullErrorUp(t *testing.T) {
	cause := errors.New("nft [add element]: exit status 1 (stderr: Operation not permitted)")
	c := &fakeCtx{args: []string{"aa:bb:cc:11:22:33"}}

	err := newBanHandler(cause).Handle(c)
	if err == nil || !errors.Is(err, cause) {
		t.Fatalf("полная ошибка должна вернуться наверх, got: %v", err)
	}
	if len(c.sent) != 1 {
		t.Fatalf("ожидалось одно сообщение юзеру, got: %v", c.sent)
	}
	if strings.Contains(c.sent[0], "stderr") || strings.Contains(c.sent[0], "exit status") {
		t.Errorf("внутренности ошибки утекли в чат: %q", c.sent[0])
	}
}

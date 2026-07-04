package handler

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tumour/openwrt-bot/internal/app/device"
	"github.com/tumour/openwrt-bot/internal/domain"
)

// stubRate — стаб device.RateLimitPort для тестов /limit и /unlimit.
type stubRate struct {
	setErr    error
	removeErr error
}

func (s *stubRate) Set(context.Context, domain.MAC, domain.Rate) error { return s.setErr }
func (s *stubRate) Remove(context.Context, domain.MAC) error           { return s.removeErr }
func (s *stubRate) List(context.Context) (map[domain.MAC]domain.Rate, error) {
	return nil, nil
}

func newLimitHandler(setErr error) *Limit {
	return NewLimit(device.NewSetLimit(&stubRate{setErr: setErr}))
}

func TestLimit_NoArgs_ShowsUsage(t *testing.T) {
	c := &fakeCtx{}
	if err := newLimitHandler(nil).Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(c.sent) != 1 || !strings.Contains(c.sent[0], "использование") {
		t.Errorf("ожидалась подсказка по использованию, got: %v", c.sent)
	}
}

func TestLimit_OneArg_ShowsUsage(t *testing.T) {
	c := &fakeCtx{args: []string{"aa:bb:cc:11:22:33"}}
	if err := newLimitHandler(nil).Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(c.sent) != 1 || !strings.Contains(c.sent[0], "использование") {
		t.Errorf("ожидалась подсказка по использованию, got: %v", c.sent)
	}
}

func TestLimit_InvalidMAC_ShowsWarning(t *testing.T) {
	c := &fakeCtx{args: []string{"not-a-mac", "512"}}
	if err := newLimitHandler(nil).Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(c.sent) != 1 || !strings.Contains(c.sent[0], "невалидный MAC") {
		t.Errorf("ожидалось предупреждение о MAC, got: %v", c.sent)
	}
}

func TestLimit_InvalidRate_ShowsWarning(t *testing.T) {
	for _, bad := range []string{"abc", "0", "-5", "1000001"} {
		c := &fakeCtx{args: []string{"aa:bb:cc:11:22:33", bad}}
		if err := newLimitHandler(nil).Handle(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(c.sent) != 1 || !strings.Contains(c.sent[0], "невалидный лимит") {
			t.Errorf("rate %q: ожидалось предупреждение о лимите, got: %v", bad, c.sent)
		}
	}
}

func TestLimit_OK_SendsConfirmation(t *testing.T) {
	c := &fakeCtx{args: []string{"AA:BB:CC:11:22:33", "512"}}
	if err := newLimitHandler(nil).Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(c.sent) != 1 || !strings.Contains(c.sent[0], "aa:bb:cc:11:22:33") ||
		!strings.Contains(c.sent[0], "512") {
		t.Errorf("ожидалось подтверждение с MAC и лимитом, got: %v", c.sent)
	}
}

// Контракт ошибок — как у Ban: юзеру коротко, полная цепочка наверх (в лог).
func TestLimit_UCError_ShortMessageFullErrorUp(t *testing.T) {
	cause := errors.New("nft [script]: exit status 1 (stderr: No such file or directory)")
	c := &fakeCtx{args: []string{"aa:bb:cc:11:22:33", "512"}}

	err := newLimitHandler(cause).Handle(c)
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

func newUnlimitHandler(removeErr error) *Unlimit {
	return NewUnlimit(device.NewRemoveLimit(&stubRate{removeErr: removeErr}))
}

func TestUnlimit_NoArgs_ShowsUsage(t *testing.T) {
	c := &fakeCtx{}
	if err := newUnlimitHandler(nil).Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(c.sent) != 1 || !strings.Contains(c.sent[0], "использование") {
		t.Errorf("ожидалась подсказка по использованию, got: %v", c.sent)
	}
}

func TestUnlimit_OK_SendsConfirmation(t *testing.T) {
	c := &fakeCtx{args: []string{"AA:BB:CC:11:22:33"}}
	if err := newUnlimitHandler(nil).Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(c.sent) != 1 || !strings.Contains(c.sent[0], "aa:bb:cc:11:22:33") {
		t.Errorf("ожидалось подтверждение с нормализованным MAC, got: %v", c.sent)
	}
}

func TestUnlimit_UCError_ShortMessageFullErrorUp(t *testing.T) {
	cause := errors.New("nft [script]: exit status 1")
	c := &fakeCtx{args: []string{"aa:bb:cc:11:22:33"}}

	err := newUnlimitHandler(cause).Handle(c)
	if err == nil || !errors.Is(err, cause) {
		t.Fatalf("полная ошибка должна вернуться наверх, got: %v", err)
	}
	if len(c.sent) != 1 || strings.Contains(c.sent[0], "exit status") {
		t.Errorf("юзеру — короткое сообщение без внутренностей, got: %v", c.sent)
	}
}

package middleware

import (
	"context"
	"testing"

	tele "gopkg.in/telebot.v3"
)

// newTeleContext — telebot.Context без сети (Offline) для юнит-тестов.
func newTeleContext(t *testing.T) tele.Context {
	t.Helper()
	b, err := tele.NewBot(tele.Settings{Offline: true})
	if err != nil {
		t.Fatalf("offline bot: %v", err)
	}
	return b.NewContext(tele.Update{})
}

func TestBaseContext_RoundTrip(t *testing.T) {
	c := newTeleContext(t)

	type key struct{}
	want := context.WithValue(context.Background(), key{}, "marker")
	PutBaseContext(c, want)

	if got := BaseContext(c); got != want {
		t.Errorf("BaseContext вернул не тот контекст: %v", got)
	}
}

// Без PutBaseContext (тесты, прямой вызов handler'а) — Background, не nil.
func TestBaseContext_DefaultsToBackground(t *testing.T) {
	c := newTeleContext(t)
	if got := BaseContext(c); got != context.Background() {
		t.Errorf("ожидался context.Background(), got %v", got)
	}
}

// Отмена базового контекста видна через производный — суть graceful shutdown.
func TestBaseContext_CancelPropagates(t *testing.T) {
	c := newTeleContext(t)
	base, cancel := context.WithCancel(context.Background())
	PutBaseContext(c, base)

	derived, stop := context.WithTimeout(BaseContext(c), 0) // имитация handler-таймаута
	defer stop()
	cancel()

	select {
	case <-derived.Done():
	default:
		t.Error("отмена базового ctx не дошла до производного")
	}
}

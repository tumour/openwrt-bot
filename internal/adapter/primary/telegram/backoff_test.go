package telegram

import (
	"testing"
	"time"
)

func TestBackoff_Sequence(t *testing.T) {
	b := newBackoff()
	want := []time.Duration{
		5 * time.Second, 10 * time.Second, 20 * time.Second,
		40 * time.Second, 60 * time.Second, 60 * time.Second, // cap
	}
	for i, w := range want {
		if got := b.next(); got != w {
			t.Errorf("next() #%d = %v, want %v", i+1, got, w)
		}
	}
}

func TestBackoff_Reset(t *testing.T) {
	b := newBackoff()
	b.next()
	b.next()
	b.reset()
	if got := b.next(); got != 5*time.Second {
		t.Errorf("после reset next() = %v, want 5s", got)
	}
}

// Малые значения (как инжектят тесты поллера/connect-фазы) работают так же.
func TestBackoff_SmallValues(t *testing.T) {
	b := backoff{min: time.Millisecond, max: 4 * time.Millisecond}
	want := []time.Duration{
		time.Millisecond, 2 * time.Millisecond, 4 * time.Millisecond, 4 * time.Millisecond,
	}
	for i, w := range want {
		if got := b.next(); got != w {
			t.Errorf("next() #%d = %v, want %v", i+1, got, w)
		}
	}
}

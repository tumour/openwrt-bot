package handler

import (
	"errors"
	"testing"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// parseLimitPayload держит контракт «битая кнопка → ошибка (toast), не паника»:
// payload приходит из callback data и в теории может быть чем угодно.
func TestParseLimitPayload(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		mac, rate, err := parseLimitPayload("aa:bb:cc:11:22:33|512")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mac.String() != "aa:bb:cc:11:22:33" || rate.KBps() != 512 {
			t.Errorf("parsed %s / %s", mac, rate)
		}
	})

	invalid := []struct {
		name string
		data string
		want error // nil = достаточно любой ошибки
	}{
		{"no separator", "aa:bb:cc:11:22:33", nil},
		{"empty", "", nil},
		{"bad mac", "not-a-mac|512", domain.ErrInvalidMAC},
		{"rate not a number", "aa:bb:cc:11:22:33|abc", nil},
		{"rate out of range", "aa:bb:cc:11:22:33|0", domain.ErrInvalidRate},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := parseLimitPayload(tc.data)
			if err == nil {
				t.Fatalf("expected error for %q", tc.data)
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

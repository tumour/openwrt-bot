package handler

import (
	"errors"
	"testing"

	"github.com/tumour/openwrt-bot/internal/domain"
)

func TestParseTimerAction(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		mac, action, err := parseTimerAction("aa:bb:cc:11:22:33|vpnoff")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mac.String() != "aa:bb:cc:11:22:33" || action != domain.ActionVPNOff {
			t.Errorf("parsed %s / %v", mac, action)
		}
	})

	invalid := []struct {
		name string
		data string
		want error // nil = достаточно любой ошибки
	}{
		{"no separator", "aa:bb:cc:11:22:33", nil},
		{"empty", "", nil},
		{"bad mac", "not-a-mac|ban", domain.ErrInvalidMAC},
		{"bad action", "aa:bb:cc:11:22:33|reboot", domain.ErrInvalidAction},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := parseTimerAction(tc.data); err == nil {
				t.Fatalf("expected error for %q", tc.data)
			} else if tc.want != nil && !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestParseTimerSet(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		mac, action, mins, err := parseTimerSet("aa:bb:cc:11:22:33|ban|45")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mac.String() != "aa:bb:cc:11:22:33" || action != domain.ActionBan || mins != 45 {
			t.Errorf("parsed %s / %v / %d", mac, action, mins)
		}
	})

	invalid := []struct {
		name string
		data string
		want error
	}{
		{"too few parts", "aa:bb:cc:11:22:33|ban", nil},
		{"empty", "", nil},
		{"bad mac", "zz|ban|45", domain.ErrInvalidMAC},
		{"bad action", "aa:bb:cc:11:22:33|nope|45", domain.ErrInvalidAction},
		{"minutes not a number", "aa:bb:cc:11:22:33|ban|abc", nil},
		{"minutes out of range", "aa:bb:cc:11:22:33|ban|0", domain.ErrInvalidMinutes},
		{"minutes above a day", "aa:bb:cc:11:22:33|ban|100000", domain.ErrInvalidMinutes},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, _, err := parseTimerSet(tc.data); err == nil {
				t.Fatalf("expected error for %q", tc.data)
			} else if tc.want != nil && !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestParseTaskID(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		id, err := parseTaskID("42")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != domain.TaskID(42) {
			t.Errorf("id = %d, want 42", id)
		}
	})
	for _, data := range []string{"", "abc", "-1", "12x"} {
		t.Run("invalid "+data, func(t *testing.T) {
			if _, err := parseTaskID(data); err == nil {
				t.Errorf("expected error for %q", data)
			}
		})
	}
}

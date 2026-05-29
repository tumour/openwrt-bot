package domain

import (
	"errors"
	"testing"
)

func TestNewMAC(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    MAC
		wantErr error
	}{
		{"lowercase canonical", "aa:bb:cc:11:22:33", "aa:bb:cc:11:22:33", nil},
		{"uppercase", "AA:BB:CC:11:22:33", "aa:bb:cc:11:22:33", nil},
		{"dash separator", "aa-bb-cc-11-22-33", "aa:bb:cc:11:22:33", nil},
		{"mixed case + dash", "Aa-Bb-Cc-11-22-33", "aa:bb:cc:11:22:33", nil},
		{"too short", "aa:bb:cc:11:22", "", ErrInvalidMAC},
		{"non-hex", "zz:bb:cc:11:22:33", "", ErrInvalidMAC},
		{"empty", "", "", ErrInvalidMAC},
		{"junk", "not a mac", "", ErrInvalidMAC},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewMAC(tc.input)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

package domain

import (
	"errors"
	"testing"
)

func TestNewRate(t *testing.T) {
	tests := []struct {
		name    string
		input   int
		want    Rate
		wantErr error
	}{
		{"minimum", 1, 1, nil},
		{"typical", 512, 512, nil},
		{"maximum", 1_000_000, 1_000_000, nil},
		{"zero is not a rate", 0, 0, ErrInvalidRate},
		{"negative", -5, 0, ErrInvalidRate},
		{"above maximum", 1_000_001, 0, ErrInvalidRate},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewRate(tc.input)
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
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestRate_IsZero(t *testing.T) {
	if !(Rate(0)).IsZero() {
		t.Error("Rate(0).IsZero() = false, want true")
	}
	if (Rate(512)).IsZero() {
		t.Error("Rate(512).IsZero() = true, want false")
	}
}

func TestRate_String(t *testing.T) {
	if got := Rate(512).String(); got != "512" {
		t.Errorf("String() = %q, want %q", got, "512")
	}
}

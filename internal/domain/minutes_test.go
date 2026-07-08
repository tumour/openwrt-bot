package domain

import (
	"errors"
	"testing"
	"time"
)

func TestNewMinutes(t *testing.T) {
	tests := []struct {
		name    string
		input   int
		want    Minutes
		wantErr error
	}{
		{"minimum", 1, 1, nil},
		{"typical", 45, 45, nil},
		{"maximum", 1440, 1440, nil},
		{"zero is not a delay", 0, 0, ErrInvalidMinutes},
		{"negative", -10, 0, ErrInvalidMinutes},
		{"above a day", 1441, 0, ErrInvalidMinutes},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewMinutes(tc.input)
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

func TestMinutes_Duration(t *testing.T) {
	if got := Minutes(45).Duration(); got != 45*time.Minute {
		t.Errorf("Duration() = %v, want %v", got, 45*time.Minute)
	}
}

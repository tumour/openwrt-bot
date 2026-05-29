package status

import (
	"context"
	"errors"
	"testing"
	"time"
)

// stubSystemPort — фейк-реализация SystemPort для изолированного теста use case.
// В hexagonal-подходе мокаются только output ports — внешние границы.
type stubSystemPort struct {
	snap Snapshot
	err  error
}

func (s stubSystemPort) Snapshot(_ context.Context) (Snapshot, error) {
	return s.snap, s.err
}

func TestGetStatus_Execute_OK(t *testing.T) {
	want := Snapshot{
		Uptime:      42 * time.Hour,
		MemTotalKB:  1024 * 1024,
		MemFreeKB:   512 * 1024,
		LoadAvg1:    0.42,
		TempCelsius: 55.3,
	}
	uc := NewGetStatus(stubSystemPort{snap: want})

	got, err := uc.Execute(context.Background(), GetStatusInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Snapshot != want {
		t.Errorf("got %+v, want %+v", got.Snapshot, want)
	}
}

func TestGetStatus_Execute_PortError(t *testing.T) {
	portErr := errors.New("boom")
	uc := NewGetStatus(stubSystemPort{err: portErr})

	_, err := uc.Execute(context.Background(), GetStatusInput{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, portErr) {
		t.Errorf("error should wrap port error; got %v", err)
	}
}

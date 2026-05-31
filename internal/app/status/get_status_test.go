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

// stubThermalPort — фейк ThermalPort: отдаёт предзаданную температуру либо ошибку.
type stubThermalPort struct {
	temp float64
	err  error
}

func (s stubThermalPort) Temperature(_ context.Context) (float64, error) {
	return s.temp, s.err
}

func TestGetStatus_Execute_OK(t *testing.T) {
	sysSnap := Snapshot{
		Uptime:     42 * time.Hour,
		MemTotalKB: 1024 * 1024,
		MemFreeKB:  512 * 1024,
		LoadAvg1:   0.42,
	}
	uc := NewGetStatus(stubSystemPort{snap: sysSnap}, stubThermalPort{temp: 55.3})

	got, err := uc.Execute(context.Background(), GetStatusInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Ожидаем системный срез + температуру, подмешанную из ThermalPort.
	want := sysSnap
	want.TempCelsius = 55.3
	if got.Snapshot != want {
		t.Errorf("got %+v, want %+v", got.Snapshot, want)
	}
}

func TestGetStatus_Execute_PortError(t *testing.T) {
	portErr := errors.New("boom")
	uc := NewGetStatus(stubSystemPort{err: portErr}, stubThermalPort{temp: 55.3})

	_, err := uc.Execute(context.Background(), GetStatusInput{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, portErr) {
		t.Errorf("error should wrap port error; got %v", err)
	}
}

// Ошибка датчика температуры (нет thermal-зоны / битый файл) не должна ронять
// /status — это best-effort метрика. Снапшот возвращается, TempCelsius остаётся 0.
func TestGetStatus_Execute_ThermalError_StillOK(t *testing.T) {
	sysSnap := Snapshot{Uptime: time.Hour, LoadAvg1: 0.1}
	uc := NewGetStatus(stubSystemPort{snap: sysSnap}, stubThermalPort{err: errors.New("no sensor")})

	got, err := uc.Execute(context.Background(), GetStatusInput{})
	if err != nil {
		t.Fatalf("thermal error must not fail status: %v", err)
	}
	if got.Snapshot.TempCelsius != 0 {
		t.Errorf("TempCelsius = %v, want 0 on sensor error", got.Snapshot.TempCelsius)
	}
	if got.Snapshot.Uptime != sysSnap.Uptime {
		t.Errorf("system snapshot lost: got %+v", got.Snapshot)
	}
}

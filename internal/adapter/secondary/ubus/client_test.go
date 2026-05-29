package ubus

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// fakeRunner — мок system.Runner. Записывает аргументы вызова и возвращает
// предзаданный stdout/error. Это позволяет тестировать adapter без реального ubus.
type fakeRunner struct {
	gotName string
	gotArgs []string
	out     []byte
	err     error
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.gotName = name
	f.gotArgs = args
	return f.out, f.err
}

func TestClient_Snapshot_OK(t *testing.T) {
	// Реальный фрагмент вывода `ubus call system info` (упрощённый).
	stub := []byte(`{
		"uptime": 86400,
		"load": [65536, 32768, 16384],
		"memory": {"total": 268435456, "free": 134217728}
	}`)
	fr := &fakeRunner{out: stub}
	c := NewClient(fr)

	snap, err := c.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Проверяем что вызвали правильную команду.
	if fr.gotName != "ubus" || strings.Join(fr.gotArgs, " ") != "call system info" {
		t.Errorf("command = %s %v, want `ubus call system info`", fr.gotName, fr.gotArgs)
	}

	// Преобразование: 86400 → 24h, load[0]=65536 → 1.0, memory в KB.
	if snap.Uptime != 24*time.Hour {
		t.Errorf("Uptime = %v, want 24h", snap.Uptime)
	}
	if snap.LoadAvg1 != 1.0 || snap.LoadAvg5 != 0.5 || snap.LoadAvg15 != 0.25 {
		t.Errorf("Load = %v/%v/%v, want 1.0/0.5/0.25", snap.LoadAvg1, snap.LoadAvg5, snap.LoadAvg15)
	}
	if snap.MemTotalKB != 262144 || snap.MemFreeKB != 131072 {
		t.Errorf("Mem = %d/%d KB, want 262144/131072", snap.MemTotalKB, snap.MemFreeKB)
	}
}

func TestClient_Snapshot_RunnerError(t *testing.T) {
	runErr := errors.New("ubus: command not found")
	c := NewClient(&fakeRunner{err: runErr})

	_, err := c.Snapshot(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, runErr) {
		t.Errorf("error should wrap runner error; got %v", err)
	}
}

func TestClient_Snapshot_InvalidJSON(t *testing.T) {
	c := NewClient(&fakeRunner{out: []byte(`{not json`)})

	_, err := c.Snapshot(context.Background())
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
}

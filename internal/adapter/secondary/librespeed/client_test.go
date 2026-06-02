package librespeed

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// fakeRunner — мок system.Runner: пишет аргументы, отдаёт предзаданный stdout/err.
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

// Реальный вывод `librespeed-cli --json`, снятый с роутера (AX6S, 2026-05-31).
const realJSON = `[{"timestamp":"2026-05-31T20:59:47.172379105Z","server":{"name":"Argalasti, Magnesia, Greece (Cosmote)","url":"https://argalasti.skoultsos.eu/"},"client":{"ip":"","hostname":"","city":"","region":"","country":"RU","loc":"","org":"","postal":"","timezone":""},"bytes_sent":48234496,"bytes_received":24426081,"ping":57.18,"jitter":0.3,"upload":24.73,"download":12.52,"share":""}]`

func TestClient_Measure_OK(t *testing.T) {
	fr := &fakeRunner{out: []byte(realJSON)}
	c := NewClient(fr, "")

	res, err := c.Measure(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fr.gotName != "librespeed-cli" || strings.Join(fr.gotArgs, " ") != "--json" {
		t.Errorf("command = %s %v, want `librespeed-cli --json`", fr.gotName, fr.gotArgs)
	}
	if res.DownloadMbps != 12.52 || res.UploadMbps != 24.73 {
		t.Errorf("speed = %v/%v Mbps, want 12.52/24.73", res.DownloadMbps, res.UploadMbps)
	}
	if res.PingMs != 57.18 || res.JitterMs != 0.3 {
		t.Errorf("ping/jitter = %v/%v, want 57.18/0.3", res.PingMs, res.JitterMs)
	}
	if res.Server != "Argalasti, Magnesia, Greece (Cosmote)" {
		t.Errorf("server = %q, want Greek server", res.Server)
	}
}

func TestClient_Measure_PinnedServer(t *testing.T) {
	fr := &fakeRunner{out: []byte(realJSON)}
	c := NewClient(fr, "42")

	if _, err := c.Measure(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Join(fr.gotArgs, " ") != "--json --server 42" {
		t.Errorf("args = %v, want `--json --server 42`", fr.gotArgs)
	}
}

func TestClient_Measure_RunnerError(t *testing.T) {
	runErr := errors.New("librespeed-cli: not found")
	c := NewClient(&fakeRunner{err: runErr}, "")

	_, err := c.Measure(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, runErr) {
		t.Errorf("error should wrap runner error; got %v", err)
	}
}

func TestClient_Measure_BinaryMissing(t *testing.T) {
	// librespeed-cli не установлен: ошибка оборачивает exec.ErrNotFound.
	c := NewClient(&fakeRunner{err: fmt.Errorf("librespeed-cli: %w", exec.ErrNotFound)}, "")

	_, err := c.Measure(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "apk add librespeed-cli") {
		t.Errorf("ожидалась подсказка про установку, got: %v", err)
	}
}

func TestClient_Measure_EmptyResult(t *testing.T) {
	c := NewClient(&fakeRunner{out: []byte(`[]`)}, "")

	if _, err := c.Measure(context.Background()); err == nil {
		t.Fatal("expected error for empty result array, got nil")
	}
}

func TestClient_Measure_InvalidJSON(t *testing.T) {
	c := NewClient(&fakeRunner{out: []byte(`{not json`)}, "")

	if _, err := c.Measure(context.Background()); err == nil {
		t.Fatal("expected parse error, got nil")
	}
}

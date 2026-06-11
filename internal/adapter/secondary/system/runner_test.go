package system

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// ExecRunner — тонкая обёртка, тестируется на 2 кейса: ok-команда и failing.
// /bin/echo и /bin/false есть на любом Linux; OpenWrt ставит busybox-варианты.

func TestExecRunner_Run_OK(t *testing.T) {
	r := NewExecRunner()
	out, err := r.Run(context.Background(), "echo", "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "hello world" {
		t.Errorf("stdout = %q, want %q", got, "hello world")
	}
}

func TestExecRunner_Run_NonZeroExit(t *testing.T) {
	r := NewExecRunner()
	_, err := r.Run(context.Background(), "false")
	if err == nil {
		t.Fatal("expected error for non-zero exit, got nil")
	}
}

// Контракт ExecError: stderr — отдельным полем (по нему типизируют ошибки
// адаптеры), и он же попадает в Error() для диагностики в логах.
func TestExecRunner_Run_StderrInExecError(t *testing.T) {
	r := NewExecRunner()
	_, err := r.Run(context.Background(), "sh", "-c", "echo boom >&2; exit 3")

	var ee *ExecError
	if !errors.As(err, &ee) {
		t.Fatalf("ожидалась *ExecError, got: %T %v", err, err)
	}
	if !strings.Contains(string(ee.Stderr), "boom") {
		t.Errorf("stderr не захвачен: %q", ee.Stderr)
	}
	if !strings.Contains(ee.Error(), "boom") {
		t.Errorf("stderr должен попадать в Error() для логов: %q", ee.Error())
	}
}

// Несуществующий бинарь: errors.Is(err, exec.ErrNotFound) обязан работать
// сквозь ExecError — на этом стоит librespeed-адаптер.
func TestExecRunner_Run_NotFoundUnwraps(t *testing.T) {
	r := NewExecRunner()
	_, err := r.Run(context.Background(), "definitely-no-such-binary-12345")
	if !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("ожидался exec.ErrNotFound сквозь ExecError, got: %v", err)
	}
}

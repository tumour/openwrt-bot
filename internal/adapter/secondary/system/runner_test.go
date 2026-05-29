package system

import (
	"context"
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

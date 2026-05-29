package system

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOSFileReader_OK(t *testing.T) {
	// Используем t.TempDir() — он автоматически чистится по окончанию теста.
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	want := []byte("hello\nworld\n")
	if err := os.WriteFile(path, want, 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	r := NewOSFileReader()
	got, err := r.ReadFile(context.Background(), path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestOSFileReader_NotFound(t *testing.T) {
	r := NewOSFileReader()
	_, err := r.ReadFile(context.Background(), "/nonexistent/file")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

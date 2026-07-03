package system

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOSFileWriter_WriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")
	w := NewOSFileWriter()
	ctx := context.Background()

	if err := w.WriteFileAtomic(ctx, path, []byte(`{"v":1}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != `{"v":1}` {
		t.Fatalf("read back: %q, %v", got, err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Errorf("perm = %o, want 600", info.Mode().Perm())
	}

	// Перезапись: та же атомарность, tmp-мусора в каталоге не остаётся.
	if err := w.WriteFileAtomic(ctx, path, []byte(`{"v":2}`), 0o600); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("в каталоге %d файлов, want 1 (tmp должен быть убран)", len(entries))
	}
}

func TestOSFileWriter_MkdirAll(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "database", "json")
	w := NewOSFileWriter()

	if err := w.MkdirAll(context.Background(), nested, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	info, err := os.Stat(nested)
	if err != nil || !info.IsDir() {
		t.Fatalf("каталог не создан: %v", err)
	}
}

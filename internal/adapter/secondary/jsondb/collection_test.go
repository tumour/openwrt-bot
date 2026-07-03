package jsondb

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/tumour/openwrt-bot/internal/adapter/secondary/system"
)

type rec struct {
	ID int `json:"id"`
}

func newTestCollection(t *testing.T) (Collection[rec], string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "recs.json")
	return NewCollection[rec](path, 1, system.NewOSFileReader(), system.NewOSFileWriter()), path
}

func TestCollection_LoadMissingFile_IsEmpty(t *testing.T) {
	c, _ := newTestCollection(t)
	items, err := c.Load(context.Background())
	if err != nil || len(items) != 0 {
		t.Fatalf("свежая установка: items=%v err=%v, want пусто без ошибки", items, err)
	}
}

func TestCollection_SaveLoad_RoundTrip(t *testing.T) {
	c, _ := newTestCollection(t)
	ctx := context.Background()

	if err := c.Save(ctx, []rec{{1}, {2}}); err != nil {
		t.Fatalf("save: %v", err)
	}
	items, err := c.Load(ctx)
	if err != nil || len(items) != 2 || items[0].ID != 1 {
		t.Fatalf("load: %v, %v", items, err)
	}
}

func TestCollection_SchemaVersionMismatch(t *testing.T) {
	c, path := newTestCollection(t)
	ctx := context.Background()
	if err := c.Save(ctx, []rec{{1}}); err != nil {
		t.Fatal(err)
	}

	// Тот же файл читает адаптер, ожидающий v2 — например, будущая версия
	// без миграции или откат бота.
	c2 := NewCollection[rec](path, 2, system.NewOSFileReader(), system.NewOSFileWriter())
	if _, err := c2.Load(ctx); !errors.Is(err, ErrSchemaVersion) {
		t.Errorf("err = %v, want ErrSchemaVersion", err)
	}
}

func TestCollection_CorruptFile_Fails(t *testing.T) {
	c, path := newTestCollection(t)
	if err := os.WriteFile(path, []byte("{битый json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Load(context.Background()); err == nil {
		t.Error("битый файл должен давать ошибку, а не пустую коллекцию")
	}
}

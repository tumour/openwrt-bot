// Package accessjson — JSON-реализация портов access.UserStore и
// access.RoleStore поверх движка jsondb: <dir>/users.json и <dir>/roles.json,
// где dir — каталог движка (…/database/json). Пакет владеет ТОЛЬКО своими
// файлами и маппингом record↔domain; вся механика хранения — в jsondb.
// Смена движка завтра = пакет accesssqlite рядом, совместимость доказывает
// общий контрактный сьют (access/accesstest).
package accessjson

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/tumour/openwrt-bot/internal/adapter/secondary/jsondb"
	"github.com/tumour/openwrt-bot/internal/adapter/secondary/system"
	"github.com/tumour/openwrt-bot/internal/app/access"
)

// schemaVersion — версия формата ЗАПИСЕЙ этого адаптера (не движка).
// Меняется при несовместимой правке userRecord/roleRecord.
const schemaVersion = 1

// Store — одно хранилище на всю фичу access: обе коллекции, один мьютекс
// (операция над users видит согласованное с roles состояние и наоборот).
// Порты наружу отдаются sub-store'ами Users()/Roles(): у одного Go-типа не
// может быть двух методов All с разными сигнатурами, а порты обязаны
// оставаться чистыми (All/Get/Put/Delete, без заикания UserStore.AllUsers).
type Store struct {
	mu    sync.Mutex
	users jsondb.Collection[userRecord]
	roles jsondb.Collection[roleRecord]
	dir   string
	w     system.FileWriter
}

func New(r system.FileReader, w system.FileWriter, dir string) *Store {
	return &Store{
		users: jsondb.NewCollection[userRecord](filepath.Join(dir, "users.json"), schemaVersion, r, w),
		roles: jsondb.NewCollection[roleRecord](filepath.Join(dir, "roles.json"), schemaVersion, r, w),
		dir:   dir,
		w:     w,
	}
}

// Users — представление хранилища как access.UserStore (методы в users.go).
func (s *Store) Users() access.UserStore { return userStore{s} }

// Roles — представление хранилища как access.RoleStore (методы в roles.go).
func (s *Store) Roles() access.RoleStore { return roleStore{s} }

// Init создаёт каталог стора (700 — внутри данные доступа). Зовётся из
// composition root до Seed.
func (s *Store) Init(ctx context.Context) error {
	return s.w.MkdirAll(ctx, s.dir, 0o700)
}

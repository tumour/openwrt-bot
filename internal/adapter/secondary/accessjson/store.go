// Package accessjson — JSON-реализация портов фичи access (Atomic +
// UserStore + RoleStore) поверх движка jsondb: <dir>/users.json и
// <dir>/roles.json, где dir — каталог движка (…/database/json). Пакет
// владеет ТОЛЬКО своими файлами и маппингом record↔domain; механика
// хранения — в jsondb. Смена движка завтра = пакет accesssqlite рядом,
// совместимость доказывает общий контрактный сьют (access/accesstest).
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

// Store — одно хранилище на всю фичу access. Единица работы — транзакция
// Update: обе коллекции читаются в память, колбэк работает со снапшотом,
// файлы перезаписываются только при успехе (ошибка = rollback, как в SQL).
// Мьютекс сериализует транзакции; одиночные операции портов — те же
// транзакции из одной операции (Users()/Roles() — тонкие обёртки).
type Store struct {
	mu    sync.Mutex
	users jsondb.Collection[userRecord]
	roles jsondb.Collection[roleRecord]
	dir   string
	w     system.FileWriter
}

// Компайл-тайм гарантия соответствия портам.
var _ access.Atomic = (*Store)(nil)

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

// Update — access.Atomic: fn выполняется над снапшотом обеих коллекций под
// мьютексом; файлы пишутся только если fn вернул nil (и только изменённые).
// ВАЖНО: внутри fn пользоваться переданными сторами — вызов s.Users()/Roles()
// из колбэка взял бы тот же мьютекс повторно (дедлок, о нём предупреждает
// и контракт порта).
func (s *Store) Update(ctx context.Context, fn func(access.UserStore, access.RoleStore) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	users, err := s.users.Load(ctx)
	if err != nil {
		return err
	}
	roles, err := s.roles.Load(ctx)
	if err != nil {
		return err
	}
	tx := &txState{users: users, roles: roles}
	if err := fn(txUsers{tx}, txRoles{tx}); err != nil {
		return err // rollback: файлы не тронуты
	}
	if tx.usersDirty {
		if err := s.users.Save(ctx, tx.users); err != nil {
			return err
		}
	}
	if tx.rolesDirty {
		if err := s.roles.Save(ctx, tx.roles); err != nil {
			return err
		}
	}
	return nil
}

// txState — рабочий снапшот транзакции: колбэк мутирует записи в памяти,
// dirty-флаги решают, какие файлы переписывать при коммите.
type txState struct {
	users      []userRecord
	roles      []roleRecord
	usersDirty bool
	rolesDirty bool
}

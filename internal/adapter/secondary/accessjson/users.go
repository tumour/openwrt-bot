package accessjson

import (
	"context"
	"fmt"

	"github.com/tumour/openwrt-bot/internal/adapter/secondary/jsondb"
	"github.com/tumour/openwrt-bot/internal/app/access"
	"github.com/tumour/openwrt-bot/internal/domain"
)

// txUsers — access.UserStore внутри транзакции: операции над снапшотом,
// файл пишет коммит Update.
type txUsers struct{ tx *txState }

var _ access.UserStore = txUsers{}

func (t txUsers) All(context.Context) ([]domain.User, error) {
	out := make([]domain.User, 0, len(t.tx.users))
	for _, rec := range t.tx.users {
		u, err := rec.toDomain()
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, nil
}

func (t txUsers) Get(_ context.Context, id domain.UserID) (domain.User, error) {
	rec, ok := jsondb.Find(t.tx.users, byUserID(id))
	if !ok {
		return domain.User{}, fmt.Errorf("%w: %d", domain.ErrUserNotFound, id)
	}
	return rec.toDomain()
}

func (t txUsers) Put(_ context.Context, u domain.User) error {
	t.tx.users = jsondb.Upsert(t.tx.users, byUserID(u.ID), userRecordFrom(u))
	t.tx.usersDirty = true
	return nil
}

func (t txUsers) Delete(_ context.Context, id domain.UserID) error {
	rest, ok := jsondb.Remove(t.tx.users, byUserID(id))
	if !ok {
		return fmt.Errorf("%w: %d", domain.ErrUserNotFound, id)
	}
	t.tx.users = rest
	t.tx.usersDirty = true
	return nil
}

// userStore — access.UserStore вне транзакции: каждая операция — транзакция
// из одного действия (тот же Update, никакой второй механики хранения).
type userStore struct{ s *Store }

var _ access.UserStore = userStore{}

func (v userStore) All(ctx context.Context) (out []domain.User, err error) {
	err = v.s.Update(ctx, func(users access.UserStore, _ access.RoleStore) error {
		out, err = users.All(ctx)
		return err
	})
	return out, err
}

func (v userStore) Get(ctx context.Context, id domain.UserID) (out domain.User, err error) {
	err = v.s.Update(ctx, func(users access.UserStore, _ access.RoleStore) error {
		out, err = users.Get(ctx, id)
		return err
	})
	return out, err
}

func (v userStore) Put(ctx context.Context, u domain.User) error {
	return v.s.Update(ctx, func(users access.UserStore, _ access.RoleStore) error {
		return users.Put(ctx, u)
	})
}

func (v userStore) Delete(ctx context.Context, id domain.UserID) error {
	return v.s.Update(ctx, func(users access.UserStore, _ access.RoleStore) error {
		return users.Delete(ctx, id)
	})
}

func byUserID(id domain.UserID) func(userRecord) bool {
	return func(rec userRecord) bool { return rec.ID == int64(id) }
}

// --- запись файла и маппинг (JSON-теги живут здесь, домен о них не знает) ---

type userRecord struct {
	ID     int64  `json:"id"`
	Name   string `json:"name,omitempty"`
	Role   string `json:"role,omitempty"`
	Status string `json:"status"`
}

func userRecordFrom(u domain.User) userRecord {
	return userRecord{ID: int64(u.ID), Name: u.Name, Role: string(u.Role), Status: string(u.Status)}
}

// toDomain валидирует запись доменными конструкторами: битые данные о доступе
// не должны молча превращаться в «каких-то» пользователей.
func (rec userRecord) toDomain() (domain.User, error) {
	id, err := domain.NewUserID(rec.ID)
	if err != nil {
		return domain.User{}, fmt.Errorf("users.json: %w", err)
	}
	switch domain.UserStatus(rec.Status) {
	case domain.StatusPending:
		return domain.NewPendingUser(id, rec.Name), nil
	case domain.StatusActive:
		role, err := domain.NewRoleName(rec.Role)
		if err != nil {
			return domain.User{}, fmt.Errorf("users.json: user %d: %w", rec.ID, err)
		}
		return domain.NewActiveUser(id, rec.Name, role), nil
	default:
		return domain.User{}, fmt.Errorf("users.json: user %d: unknown status %q", rec.ID, rec.Status)
	}
}

package accessjson

import (
	"context"
	"fmt"

	"github.com/tumour/openwrt-bot/internal/adapter/secondary/jsondb"
	"github.com/tumour/openwrt-bot/internal/app/access"
	"github.com/tumour/openwrt-bot/internal/domain"
)

// userStore — sub-store: реализация access.UserStore поверх общего Store
// (коллекция users.json под общим локом).
type userStore struct{ s *Store }

var _ access.UserStore = userStore{}

func (v userStore) All(ctx context.Context) ([]domain.User, error) {
	v.s.mu.Lock()
	defer v.s.mu.Unlock()
	recs, err := v.s.users.Load(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.User, 0, len(recs))
	for _, rec := range recs {
		u, err := rec.toDomain()
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, nil
}

func (v userStore) Get(ctx context.Context, id domain.UserID) (domain.User, error) {
	v.s.mu.Lock()
	defer v.s.mu.Unlock()
	recs, err := v.s.users.Load(ctx)
	if err != nil {
		return domain.User{}, err
	}
	rec, ok := jsondb.Find(recs, byUserID(id))
	if !ok {
		return domain.User{}, fmt.Errorf("%w: %d", domain.ErrUserNotFound, id)
	}
	return rec.toDomain()
}

func (v userStore) Put(ctx context.Context, u domain.User) error {
	v.s.mu.Lock()
	defer v.s.mu.Unlock()
	recs, err := v.s.users.Load(ctx)
	if err != nil {
		return err
	}
	return v.s.users.Save(ctx, jsondb.Upsert(recs, byUserID(u.ID), userRecordFrom(u)))
}

func (v userStore) Delete(ctx context.Context, id domain.UserID) error {
	v.s.mu.Lock()
	defer v.s.mu.Unlock()
	recs, err := v.s.users.Load(ctx)
	if err != nil {
		return err
	}
	rest, ok := jsondb.Remove(recs, byUserID(id))
	if !ok {
		return fmt.Errorf("%w: %d", domain.ErrUserNotFound, id)
	}
	return v.s.users.Save(ctx, rest)
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

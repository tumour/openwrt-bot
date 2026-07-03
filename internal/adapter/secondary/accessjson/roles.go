package accessjson

import (
	"context"
	"fmt"

	"github.com/tumour/openwrt-bot/internal/adapter/secondary/jsondb"
	"github.com/tumour/openwrt-bot/internal/app/access"
	"github.com/tumour/openwrt-bot/internal/domain"
)

// roleStore — sub-store: реализация access.RoleStore поверх общего Store
// (коллекция roles.json под общим локом).
type roleStore struct{ s *Store }

var _ access.RoleStore = roleStore{}

func (v roleStore) All(ctx context.Context) ([]domain.Role, error) {
	v.s.mu.Lock()
	defer v.s.mu.Unlock()
	recs, err := v.s.roles.Load(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Role, 0, len(recs))
	for _, rec := range recs {
		r, err := rec.toDomain()
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func (v roleStore) Get(ctx context.Context, name domain.RoleName) (domain.Role, error) {
	v.s.mu.Lock()
	defer v.s.mu.Unlock()
	recs, err := v.s.roles.Load(ctx)
	if err != nil {
		return domain.Role{}, err
	}
	rec, ok := jsondb.Find(recs, byRoleName(name))
	if !ok {
		return domain.Role{}, fmt.Errorf("%w: %q", domain.ErrRoleNotFound, name)
	}
	return rec.toDomain()
}

func (v roleStore) Put(ctx context.Context, r domain.Role) error {
	v.s.mu.Lock()
	defer v.s.mu.Unlock()
	recs, err := v.s.roles.Load(ctx)
	if err != nil {
		return err
	}
	return v.s.roles.Save(ctx, jsondb.Upsert(recs, byRoleName(r.Name), roleRecordFrom(r)))
}

func (v roleStore) Delete(ctx context.Context, name domain.RoleName) error {
	v.s.mu.Lock()
	defer v.s.mu.Unlock()
	recs, err := v.s.roles.Load(ctx)
	if err != nil {
		return err
	}
	rest, ok := jsondb.Remove(recs, byRoleName(name))
	if !ok {
		return fmt.Errorf("%w: %q", domain.ErrRoleNotFound, name)
	}
	return v.s.roles.Save(ctx, rest)
}

func byRoleName(name domain.RoleName) func(roleRecord) bool {
	return func(rec roleRecord) bool { return rec.Name == string(name) }
}

// --- запись файла и маппинг ---

type roleRecord struct {
	Name        string   `json:"name"`
	Permissions []string `json:"permissions,omitempty"`
}

func roleRecordFrom(r domain.Role) roleRecord {
	perms := make([]string, 0, len(r.Permissions))
	for _, p := range r.Permissions {
		perms = append(perms, string(p))
	}
	return roleRecord{Name: string(r.Name), Permissions: perms}
}

// toDomain: неизвестное право в файле ПРОПУСКАЕТСЯ, а не валит загрузку —
// это штатное сжатие каталога (право убрали из кода вместе с его проверкой,
// запись в данных больше ничего не значит). Невалидное имя роли — ошибка.
func (rec roleRecord) toDomain() (domain.Role, error) {
	name, err := domain.NewRoleName(rec.Name)
	if err != nil {
		return domain.Role{}, fmt.Errorf("roles.json: %w", err)
	}
	role := domain.Role{Name: name}
	for _, raw := range rec.Permissions {
		p, err := domain.NewPermission(raw)
		if err != nil {
			continue
		}
		role.Grant(p)
	}
	return role, nil
}

package accessjson

import (
	"context"
	"fmt"

	"github.com/tumour/openwrt-bot/internal/adapter/secondary/jsondb"
	"github.com/tumour/openwrt-bot/internal/app/access"
	"github.com/tumour/openwrt-bot/internal/domain"
)

// txRoles — access.RoleStore внутри транзакции: операции над снапшотом,
// файл пишет коммит Update.
type txRoles struct{ tx *txState }

var _ access.RoleStore = txRoles{}

func (t txRoles) All(context.Context) ([]domain.Role, error) {
	out := make([]domain.Role, 0, len(t.tx.roles))
	for _, rec := range t.tx.roles {
		r, err := rec.toDomain()
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func (t txRoles) Get(_ context.Context, name domain.RoleName) (domain.Role, error) {
	rec, ok := jsondb.Find(t.tx.roles, byRoleName(name))
	if !ok {
		return domain.Role{}, fmt.Errorf("%w: %q", domain.ErrRoleNotFound, name)
	}
	return rec.toDomain()
}

func (t txRoles) Put(_ context.Context, r domain.Role) error {
	t.tx.roles = jsondb.Upsert(t.tx.roles, byRoleName(r.Name), roleRecordFrom(r))
	t.tx.rolesDirty = true
	return nil
}

func (t txRoles) Delete(_ context.Context, name domain.RoleName) error {
	rest, ok := jsondb.Remove(t.tx.roles, byRoleName(name))
	if !ok {
		return fmt.Errorf("%w: %q", domain.ErrRoleNotFound, name)
	}
	t.tx.roles = rest
	t.tx.rolesDirty = true
	return nil
}

// roleStore — access.RoleStore вне транзакции: каждая операция — транзакция
// из одного действия.
type roleStore struct{ s *Store }

var _ access.RoleStore = roleStore{}

func (v roleStore) All(ctx context.Context) (out []domain.Role, err error) {
	err = v.s.Update(ctx, func(_ access.UserStore, roles access.RoleStore) error {
		out, err = roles.All(ctx)
		return err
	})
	return out, err
}

func (v roleStore) Get(ctx context.Context, name domain.RoleName) (out domain.Role, err error) {
	err = v.s.Update(ctx, func(_ access.UserStore, roles access.RoleStore) error {
		out, err = roles.Get(ctx, name)
		return err
	})
	return out, err
}

func (v roleStore) Put(ctx context.Context, r domain.Role) error {
	return v.s.Update(ctx, func(_ access.UserStore, roles access.RoleStore) error {
		return roles.Put(ctx, r)
	})
}

func (v roleStore) Delete(ctx context.Context, name domain.RoleName) error {
	return v.s.Update(ctx, func(_ access.UserStore, roles access.RoleStore) error {
		return roles.Delete(ctx, name)
	})
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

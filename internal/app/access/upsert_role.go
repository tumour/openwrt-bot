package access

import (
	"context"
	"fmt"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// UpsertRole — use case «создать/изменить роль». Только носитель manage_users.
// В этом боте у use case нет Telegram-ручки (роли правят из кода бутстрапа),
// но он часть скелета: продукт с редактором ролей выставит его одной командой.
// Guard: правка не должна оставить бота без активного носителя manage_users —
// например, снятие manage_users с роли admin, когда все админы сидят на ней.
type UpsertRole struct {
	store Atomic
}

func NewUpsertRole(store Atomic) *UpsertRole {
	return &UpsertRole{store: store}
}

type (
	UpsertRoleInput struct {
		ActorID     domain.UserID
		Name        string   // сырое имя — валидируется доменом
		Permissions []string // сырые права — валидируются по каталогу
	}
	UpsertRoleOutput struct {
		Role domain.Role
	}
)

func (uc *UpsertRole) Execute(ctx context.Context, in UpsertRoleInput) (UpsertRoleOutput, error) {
	var out UpsertRoleOutput
	err := uc.store.Update(ctx, func(users UserStore, roles RoleStore) error {
		if err := requireManager(ctx, users, roles, in.ActorID); err != nil {
			return err
		}
		role, err := domain.NewRole(in.Name, in.Permissions)
		if err != nil {
			return err
		}
		if !role.Has(domain.PermManageUsers) {
			if err := ensureManagerSurvives(ctx, users, roles, role); err != nil {
				return err
			}
		}
		if err := roles.Put(ctx, role); err != nil {
			return fmt.Errorf("put role %q: %w", role.Name, err)
		}
		out = UpsertRoleOutput{Role: role}
		return nil
	})
	return out, err
}

// ensureManagerSurvives проверяет, что после применения updated хотя бы один
// активный пользователь всё ещё носит manage_users (его эффективная роль —
// updated, если имена совпали, иначе сохранённая).
func ensureManagerSurvives(ctx context.Context, users UserStore, roles RoleStore, updated domain.Role) error {
	all, err := users.All(ctx)
	if err != nil {
		return fmt.Errorf("list users: %w", err)
	}
	rs, err := roles.All(ctx)
	if err != nil {
		return fmt.Errorf("list roles: %w", err)
	}
	byName := make(map[domain.RoleName]domain.Role, len(rs))
	for _, r := range rs {
		byName[r.Name] = r
	}
	byName[updated.Name] = updated
	for _, u := range all {
		if u.IsActive() && byName[u.Role].Has(domain.PermManageUsers) {
			return nil
		}
	}
	return fmt.Errorf("role %q update: %w", updated.Name, domain.ErrLastAdmin)
}

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
	users UserStore
	roles RoleStore
}

func NewUpsertRole(users UserStore, roles RoleStore) *UpsertRole {
	return &UpsertRole{users: users, roles: roles}
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
	if err := requireManager(ctx, uc.users, uc.roles, in.ActorID); err != nil {
		return UpsertRoleOutput{}, err
	}
	role, err := domain.NewRole(in.Name, in.Permissions)
	if err != nil {
		return UpsertRoleOutput{}, err
	}
	if !role.Has(domain.PermManageUsers) {
		if err := uc.ensureManagerSurvives(ctx, role); err != nil {
			return UpsertRoleOutput{}, err
		}
	}
	if err := uc.roles.Put(ctx, role); err != nil {
		return UpsertRoleOutput{}, fmt.Errorf("put role %q: %w", role.Name, err)
	}
	return UpsertRoleOutput{Role: role}, nil
}

// ensureManagerSurvives проверяет, что после применения updated хотя бы один
// активный пользователь всё ещё носит manage_users (его эффективная роль —
// updated, если имена совпали, иначе сохранённая).
func (uc *UpsertRole) ensureManagerSurvives(ctx context.Context, updated domain.Role) error {
	users, err := uc.users.All(ctx)
	if err != nil {
		return fmt.Errorf("list users: %w", err)
	}
	roles, err := uc.roles.All(ctx)
	if err != nil {
		return fmt.Errorf("list roles: %w", err)
	}
	byName := make(map[domain.RoleName]domain.Role, len(roles))
	for _, r := range roles {
		byName[r.Name] = r
	}
	byName[updated.Name] = updated
	for _, u := range users {
		if u.IsActive() && byName[u.Role].Has(domain.PermManageUsers) {
			return nil
		}
	}
	return fmt.Errorf("role %q update: %w", updated.Name, domain.ErrLastAdmin)
}

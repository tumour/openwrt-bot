package access

import (
	"context"
	"fmt"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// SetRole — use case «сменить роль пользователя». Только носитель manage_users.
// Guard: нельзя разжаловать последнего носителя manage_users (ErrLastAdmin) —
// бот не должен оставаться неуправляемым.
type SetRole struct {
	users UserStore
	roles RoleStore
}

func NewSetRole(users UserStore, roles RoleStore) *SetRole {
	return &SetRole{users: users, roles: roles}
}

type (
	SetRoleInput struct {
		ActorID  domain.UserID
		TargetID domain.UserID
		Role     domain.RoleName
	}
	SetRoleOutput struct {
		User domain.User // с новой ролью — для уведомления/перерисовки
	}
)

func (uc *SetRole) Execute(ctx context.Context, in SetRoleInput) (SetRoleOutput, error) {
	if err := requireManager(ctx, uc.users, uc.roles, in.ActorID); err != nil {
		return SetRoleOutput{}, err
	}
	role, err := uc.roles.Get(ctx, in.Role)
	if err != nil {
		return SetRoleOutput{}, fmt.Errorf("role %q: %w", in.Role, err)
	}
	u, err := uc.users.Get(ctx, in.TargetID)
	if err != nil {
		return SetRoleOutput{}, fmt.Errorf("get user %d: %w", in.TargetID, err)
	}
	if !u.IsActive() {
		return SetRoleOutput{}, fmt.Errorf("%w: user %d is %q", domain.ErrNotActive, u.ID, u.Status)
	}
	if u.Role == in.Role {
		return SetRoleOutput{User: u}, nil // no-op: роль уже та
	}
	// Разжалование: цель теряет manage_users → она не должна быть последней.
	if !role.Has(domain.PermManageUsers) {
		if err := ensureNotLastManager(ctx, uc.users, uc.roles, u.ID); err != nil {
			return SetRoleOutput{}, err
		}
	}
	u.Role = in.Role
	if err := uc.users.Put(ctx, u); err != nil {
		return SetRoleOutput{}, fmt.Errorf("put user %d: %w", u.ID, err)
	}
	return SetRoleOutput{User: u}, nil
}

package access

import (
	"context"
	"fmt"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// SetRole — use case «сменить роль пользователя». Только носитель manage_users.
// Guard: нельзя разжаловать последнего носителя manage_users (ErrLastAdmin) —
// бот не должен оставаться неуправляемым. Guard и запись — одна атомарная
// единица: два одновременных разжалования не проскочат мимо друг друга.
type SetRole struct {
	store Atomic
}

func NewSetRole(store Atomic) *SetRole {
	return &SetRole{store: store}
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
	var out SetRoleOutput
	err := uc.store.Update(ctx, func(users UserStore, roles RoleStore) error {
		if err := requireManager(ctx, users, roles, in.ActorID); err != nil {
			return err
		}
		role, err := roles.Get(ctx, in.Role)
		if err != nil {
			return fmt.Errorf("role %q: %w", in.Role, err)
		}
		u, err := users.Get(ctx, in.TargetID)
		if err != nil {
			return fmt.Errorf("get user %d: %w", in.TargetID, err)
		}
		if !u.IsActive() {
			return fmt.Errorf("%w: user %d is %q", domain.ErrNotActive, u.ID, u.Status)
		}
		if u.Role == in.Role {
			out = SetRoleOutput{User: u}
			return nil // no-op: роль уже та
		}
		// Разжалование: цель теряет manage_users → она не должна быть последней.
		if !role.Has(domain.PermManageUsers) {
			if err := ensureNotLastManager(ctx, users, roles, u.ID); err != nil {
				return err
			}
		}
		u.Role = in.Role
		if err := users.Put(ctx, u); err != nil {
			return fmt.Errorf("put user %d: %w", u.ID, err)
		}
		out = SetRoleOutput{User: u}
		return nil
	})
	return out, err
}

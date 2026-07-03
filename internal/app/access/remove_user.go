package access

import (
	"context"
	"fmt"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// RemoveUser — use case «удалить пользователя» (активного или заявку).
// Только носитель manage_users. Guard: последнего носителя manage_users
// удалить нельзя (ErrLastAdmin) — в т.ч. самого себя.
type RemoveUser struct {
	users UserStore
	roles RoleStore
}

func NewRemoveUser(users UserStore, roles RoleStore) *RemoveUser {
	return &RemoveUser{users: users, roles: roles}
}

type (
	RemoveUserInput struct {
		ActorID  domain.UserID
		TargetID domain.UserID
	}
	RemoveUserOutput struct {
		User domain.User // удалённый — для уведомления
	}
)

func (uc *RemoveUser) Execute(ctx context.Context, in RemoveUserInput) (RemoveUserOutput, error) {
	if err := requireManager(ctx, uc.users, uc.roles, in.ActorID); err != nil {
		return RemoveUserOutput{}, err
	}
	u, err := uc.users.Get(ctx, in.TargetID)
	if err != nil {
		return RemoveUserOutput{}, fmt.Errorf("get user %d: %w", in.TargetID, err)
	}
	if u.IsActive() {
		if err := ensureNotLastManager(ctx, uc.users, uc.roles, u.ID); err != nil {
			return RemoveUserOutput{}, err
		}
	}
	if err := uc.users.Delete(ctx, in.TargetID); err != nil {
		return RemoveUserOutput{}, fmt.Errorf("delete user %d: %w", in.TargetID, err)
	}
	return RemoveUserOutput{User: u}, nil
}

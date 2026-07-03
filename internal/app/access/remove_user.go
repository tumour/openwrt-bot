package access

import (
	"context"
	"fmt"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// RemoveUser — use case «удалить пользователя» (активного или заявку).
// Только носитель manage_users. Guard: последнего носителя manage_users
// удалить нельзя (ErrLastAdmin) — в т.ч. самого себя. Guard и удаление —
// одна атомарная единица: два админа, одновременно удаляющие друг друга,
// не оставят бота без управления (второй получит ErrLastAdmin).
type RemoveUser struct {
	store Atomic
}

func NewRemoveUser(store Atomic) *RemoveUser {
	return &RemoveUser{store: store}
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
	var out RemoveUserOutput
	err := uc.store.Update(ctx, func(users UserStore, roles RoleStore) error {
		if err := requireManager(ctx, users, roles, in.ActorID); err != nil {
			return err
		}
		u, err := users.Get(ctx, in.TargetID)
		if err != nil {
			return fmt.Errorf("get user %d: %w", in.TargetID, err)
		}
		if u.IsActive() {
			if err := ensureNotLastManager(ctx, users, roles, u.ID); err != nil {
				return err
			}
		}
		if err := users.Delete(ctx, in.TargetID); err != nil {
			return fmt.Errorf("delete user %d: %w", in.TargetID, err)
		}
		out = RemoveUserOutput{User: u}
		return nil
	})
	return out, err
}

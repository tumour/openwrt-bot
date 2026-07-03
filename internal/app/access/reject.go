package access

import (
	"context"
	"fmt"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// Reject — use case «отклонить заявку». Только носитель manage_users.
// Отклонение = удаление pending-записи: человек сможет попроситься снова.
type Reject struct {
	store Atomic
}

func NewReject(store Atomic) *Reject {
	return &Reject{store: store}
}

type (
	RejectInput struct {
		ActorID  domain.UserID
		TargetID domain.UserID
	}
	RejectOutput struct {
		User domain.User // отклонённый — для уведомления
	}
)

func (uc *Reject) Execute(ctx context.Context, in RejectInput) (RejectOutput, error) {
	var out RejectOutput
	err := uc.store.Update(ctx, func(users UserStore, roles RoleStore) error {
		if err := requireManager(ctx, users, roles, in.ActorID); err != nil {
			return err
		}
		u, err := users.Get(ctx, in.TargetID)
		if err != nil {
			return fmt.Errorf("get user %d: %w", in.TargetID, err)
		}
		if u.IsActive() {
			// Активных не «отклоняют» — их удаляют (RemoveUser, с guard'ом
			// последнего админа). Разные операции — разные инварианты.
			return fmt.Errorf("%w: user %d is %q", domain.ErrNotPending, u.ID, u.Status)
		}
		if err := users.Delete(ctx, in.TargetID); err != nil {
			return fmt.Errorf("delete user %d: %w", in.TargetID, err)
		}
		out = RejectOutput{User: u}
		return nil
	})
	return out, err
}

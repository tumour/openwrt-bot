package access

import (
	"context"
	"fmt"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// Reject — use case «отклонить заявку». Только носитель manage_users.
// Отклонение = удаление pending-записи: человек сможет попроситься снова.
type Reject struct {
	users UserStore
	roles RoleStore
}

func NewReject(users UserStore, roles RoleStore) *Reject {
	return &Reject{users: users, roles: roles}
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
	if err := requireManager(ctx, uc.users, uc.roles, in.ActorID); err != nil {
		return RejectOutput{}, err
	}
	u, err := uc.users.Get(ctx, in.TargetID)
	if err != nil {
		return RejectOutput{}, fmt.Errorf("get user %d: %w", in.TargetID, err)
	}
	if u.IsActive() {
		// Активных не «отклоняют» — их удаляют (RemoveUser, с guard'ом
		// последнего админа). Разные операции — разные инварианты.
		return RejectOutput{}, fmt.Errorf("%w: user %d is %q", domain.ErrNotPending, u.ID, u.Status)
	}
	if err := uc.users.Delete(ctx, in.TargetID); err != nil {
		return RejectOutput{}, fmt.Errorf("delete user %d: %w", in.TargetID, err)
	}
	return RejectOutput{User: u}, nil
}

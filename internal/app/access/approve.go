package access

import (
	"context"
	"fmt"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// Approve — use case «одобрить заявку». Только носитель manage_users.
// Одобренный получает встроенную роль user (дефолт решения №2).
type Approve struct {
	users UserStore
	roles RoleStore
}

func NewApprove(users UserStore, roles RoleStore) *Approve {
	return &Approve{users: users, roles: roles}
}

type (
	ApproveInput struct {
		ActorID  domain.UserID
		TargetID domain.UserID
	}
	ApproveOutput struct {
		User domain.User // одобренный — для уведомления «доступ выдан»
	}
)

func (uc *Approve) Execute(ctx context.Context, in ApproveInput) (ApproveOutput, error) {
	if err := requireManager(ctx, uc.users, uc.roles, in.ActorID); err != nil {
		return ApproveOutput{}, err
	}
	// Роль-дефолт должна существовать ДО мутации юзера — иначе получили бы
	// активного юзера с висячей ролью.
	if _, err := uc.roles.Get(ctx, domain.RoleUser); err != nil {
		return ApproveOutput{}, fmt.Errorf("default role %q: %w", domain.RoleUser, err)
	}
	u, err := uc.users.Get(ctx, in.TargetID)
	if err != nil {
		return ApproveOutput{}, fmt.Errorf("get user %d: %w", in.TargetID, err)
	}
	if err := u.Approve(domain.RoleUser); err != nil {
		return ApproveOutput{}, err
	}
	if err := uc.users.Put(ctx, u); err != nil {
		return ApproveOutput{}, fmt.Errorf("put user %d: %w", u.ID, err)
	}
	return ApproveOutput{User: u}, nil
}

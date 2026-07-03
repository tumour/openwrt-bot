package access

import (
	"context"
	"fmt"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// Approve — use case «одобрить заявку». Только носитель manage_users.
// Одобренный получает встроенную роль user (дефолт решения №2).
type Approve struct {
	store Atomic
}

func NewApprove(store Atomic) *Approve {
	return &Approve{store: store}
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
	var out ApproveOutput
	err := uc.store.Update(ctx, func(users UserStore, roles RoleStore) error {
		if err := requireManager(ctx, users, roles, in.ActorID); err != nil {
			return err
		}
		// Роль-дефолт должна существовать ДО мутации юзера — иначе получили бы
		// активного юзера с висячей ролью.
		if _, err := roles.Get(ctx, domain.RoleUser); err != nil {
			return fmt.Errorf("default role %q: %w", domain.RoleUser, err)
		}
		u, err := users.Get(ctx, in.TargetID)
		if err != nil {
			return fmt.Errorf("get user %d: %w", in.TargetID, err)
		}
		if err := u.Approve(domain.RoleUser); err != nil {
			return err
		}
		if err := users.Put(ctx, u); err != nil {
			return fmt.Errorf("put user %d: %w", u.ID, err)
		}
		out = ApproveOutput{User: u}
		return nil
	})
	return out, err
}

package access

import (
	"context"
	"errors"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// Check — use case «допущен ли отправитель и что ему можно». Зовётся
// auth-middleware на каждый апдейт, поэтому вход — сырой Telegram ID:
// невалидный/неизвестный/pending — это не ошибки, а штатное «не допущен».
type Check struct {
	users UserStore
	roles RoleStore
}

func NewCheck(users UserStore, roles RoleStore) *Check {
	return &Check{users: users, roles: roles}
}

type (
	CheckInput struct {
		UserID int64
	}
	CheckOutput struct {
		Allowed bool
		Grant   Grant // заполнен только при Allowed
	}
)

func (uc *Check) Execute(ctx context.Context, in CheckInput) (CheckOutput, error) {
	id, err := domain.NewUserID(in.UserID)
	if err != nil {
		return CheckOutput{}, nil // группы/каналы не допускаются — молча
	}
	g, err := grantOf(ctx, uc.users, uc.roles, id)
	if err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			return CheckOutput{}, nil
		}
		return CheckOutput{}, err // инфраструктурная ошибка стора — наверх
	}
	return CheckOutput{Allowed: true, Grant: g}, nil
}

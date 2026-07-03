package access

import (
	"context"
	"errors"
	"fmt"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// RequestAccess — use case «незнакомец просит доступ» (/start). Application
// rule: повторная заявка (и /start от уже допущенного) = no-op — админа не
// спамим, Created=false. Admins в выводе — кого уведомить о новой заявке.
type RequestAccess struct {
	store Atomic
}

func NewRequestAccess(store Atomic) *RequestAccess {
	return &RequestAccess{store: store}
}

type (
	RequestAccessInput struct {
		UserID int64
		Name   string // display-имя из Telegram, для карточки заявки
	}
	RequestAccessOutput struct {
		Created bool
		User    domain.User   // заявка (заполнена при Created)
		Admins  []domain.User // кого уведомить (заполнен при Created)
	}
)

func (uc *RequestAccess) Execute(ctx context.Context, in RequestAccessInput) (RequestAccessOutput, error) {
	id, err := domain.NewUserID(in.UserID)
	if err != nil {
		return RequestAccessOutput{}, err
	}
	var out RequestAccessOutput
	err = uc.store.Update(ctx, func(users UserStore, roles RoleStore) error {
		switch _, err := users.Get(ctx, id); {
		case err == nil:
			return nil // уже есть (pending или active) — дедуп
		case !errors.Is(err, domain.ErrUserNotFound):
			return fmt.Errorf("get user %d: %w", id, err)
		}

		u := domain.NewPendingUser(id, in.Name)
		if err := users.Put(ctx, u); err != nil {
			return fmt.Errorf("put pending user %d: %w", id, err)
		}
		admins, err := managers(ctx, users, roles)
		if err != nil {
			return err
		}
		out = RequestAccessOutput{Created: true, User: u, Admins: admins}
		return nil
	})
	return out, err
}

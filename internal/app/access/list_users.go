package access

import (
	"context"
	"sort"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// ListUsers — use case «список пользователей» для /users. Только носитель
// manage_users: список раскрывает ID и заявки.
type ListUsers struct {
	users UserStore
	roles RoleStore
}

func NewListUsers(users UserStore, roles RoleStore) *ListUsers {
	return &ListUsers{users: users, roles: roles}
}

type (
	ListUsersInput struct {
		ActorID domain.UserID
	}
	ListUsersOutput struct {
		Users []domain.User // заявки первыми (админу видно, что ждёт решения), затем по ID
	}
)

func (uc *ListUsers) Execute(ctx context.Context, in ListUsersInput) (ListUsersOutput, error) {
	if err := requireManager(ctx, uc.users, uc.roles, in.ActorID); err != nil {
		return ListUsersOutput{}, err
	}
	all, err := uc.users.All(ctx)
	if err != nil {
		return ListUsersOutput{}, err
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].Status != all[j].Status {
			return all[i].Status == domain.StatusPending
		}
		return all[i].ID < all[j].ID
	})
	return ListUsersOutput{Users: all}, nil
}

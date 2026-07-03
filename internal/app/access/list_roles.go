package access

import (
	"context"
	"sort"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// ListRoles — use case «какие роли существуют» (для кнопок смены роли).
// Только носитель manage_users.
type ListRoles struct {
	users UserStore
	roles RoleStore
}

func NewListRoles(users UserStore, roles RoleStore) *ListRoles {
	return &ListRoles{users: users, roles: roles}
}

type (
	ListRolesInput struct {
		ActorID domain.UserID
	}
	ListRolesOutput struct {
		Roles []domain.Role // отсортированы по имени
	}
)

func (uc *ListRoles) Execute(ctx context.Context, in ListRolesInput) (ListRolesOutput, error) {
	if err := requireManager(ctx, uc.users, uc.roles, in.ActorID); err != nil {
		return ListRolesOutput{}, err
	}
	all, err := uc.roles.All(ctx)
	if err != nil {
		return ListRolesOutput{}, err
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Name < all[j].Name })
	return ListRolesOutput{Roles: all}, nil
}

package access

import (
	"context"
	"errors"
	"fmt"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// Grant — «пропуск» пользователя: он сам и его роль с правами. Middleware
// кладёт Grant в контекст запроса, handlers и presenters решают по нему,
// что выполнять и что рисовать (нет права = нет кнопки).
type Grant struct {
	User domain.User
	Role domain.Role
}

// Has сообщает, есть ли у пользователя право.
func (g Grant) Has(p domain.Permission) bool { return g.Role.Has(p) }

// grantOf собирает Grant активного пользователя. Не активен или не найден —
// domain.ErrForbidden; активный юзер с несуществующей ролью — тоже ErrForbidden
// (без роли нет прав), но обёрнутый ErrRoleNotFound сохраняется для логов.
func grantOf(ctx context.Context, users UserStore, roles RoleStore, id domain.UserID) (Grant, error) {
	u, err := users.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return Grant{}, fmt.Errorf("user %d: %w", id, domain.ErrForbidden)
		}
		return Grant{}, fmt.Errorf("get user %d: %w", id, err)
	}
	if !u.IsActive() {
		return Grant{}, fmt.Errorf("user %d is %q: %w", id, u.Status, domain.ErrForbidden)
	}
	r, err := roles.Get(ctx, u.Role)
	if err != nil {
		if errors.Is(err, domain.ErrRoleNotFound) {
			return Grant{}, fmt.Errorf("user %d role %q: %w: %w", id, u.Role, err, domain.ErrForbidden)
		}
		return Grant{}, fmt.Errorf("get role %q: %w", u.Role, err)
	}
	return Grant{User: u, Role: r}, nil
}

// requireManager проверяет, что актор — активный носитель manage_users.
// Все use cases, меняющие список пользователей/ролей, зовут его первым:
// авторизация мутаций — правило приложения, а не вежливость handler'а.
func requireManager(ctx context.Context, users UserStore, roles RoleStore, actor domain.UserID) error {
	g, err := grantOf(ctx, users, roles, actor)
	if err != nil {
		return err
	}
	if !g.Has(domain.PermManageUsers) {
		return fmt.Errorf("user %d lacks %q: %w", actor, domain.PermManageUsers, domain.ErrForbidden)
	}
	return nil
}

// managers возвращает активных носителей manage_users. Используется для
// уведомлений о заявках и для guard'а «не остаться без админа».
func managers(ctx context.Context, users UserStore, roles RoleStore) ([]domain.User, error) {
	all, err := users.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	rs, err := roles.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	byName := make(map[domain.RoleName]domain.Role, len(rs))
	for _, r := range rs {
		byName[r.Name] = r
	}
	var out []domain.User
	for _, u := range all {
		if u.IsActive() && byName[u.Role].Has(domain.PermManageUsers) {
			out = append(out, u)
		}
	}
	return out, nil
}

// ensureNotLastManager возвращает domain.ErrLastAdmin, если после исключения
// пользователя excluded активных носителей manage_users не останется.
func ensureNotLastManager(ctx context.Context, users UserStore, roles RoleStore, excluded domain.UserID) error {
	ms, err := managers(ctx, users, roles)
	if err != nil {
		return err
	}
	for _, m := range ms {
		if m.ID != excluded {
			return nil
		}
	}
	return fmt.Errorf("user %d: %w", excluded, domain.ErrLastAdmin)
}

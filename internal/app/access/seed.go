package access

import (
	"context"
	"errors"
	"fmt"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// Seed — bootstrap доступа при старте бота (зовётся из run(), не из Telegram).
// Гарантии после выполнения:
//   - встроенные роли admin/user существуют (отсутствующая пересоздаётся);
//   - admin владеет ВСЕМ каталогом прав — «политика догона»: новое право,
//     появившееся в коде, дополняет роль-данные при первом же старте, иначе
//     существующие деплои остались бы без нового функционала;
//   - env-админ существует и активен: нет — создаётся с ролью admin,
//     есть — не трогается (env не источник правды после первого старта).
//
// Кастомные роли и остальные пользователи не трогаются никогда.
type Seed struct {
	users UserStore
	roles RoleStore
}

func NewSeed(users UserStore, roles RoleStore) *Seed {
	return &Seed{users: users, roles: roles}
}

type SeedInput struct {
	AdminID int64 // из env ADMIN_USER_ID; невалидный — ошибка, бот не стартует
}

func (uc *Seed) Execute(ctx context.Context, in SeedInput) error {
	adminID, err := domain.NewUserID(in.AdminID)
	if err != nil {
		return fmt.Errorf("ADMIN_USER_ID: %w", err)
	}

	for _, def := range domain.DefaultRoles() {
		if err := uc.ensureRole(ctx, def); err != nil {
			return err
		}
	}

	switch _, err := uc.users.Get(ctx, adminID); {
	case err == nil:
		return nil // есть — до свидания: env после первого старта не источник правды
	case !errors.Is(err, domain.ErrUserNotFound):
		return fmt.Errorf("get admin %d: %w", adminID, err)
	}
	admin := domain.NewActiveUser(adminID, "", domain.RoleAdmin)
	if err := uc.users.Put(ctx, admin); err != nil {
		return fmt.Errorf("seed admin %d: %w", adminID, err)
	}
	return nil
}

// ensureRole создаёт отсутствующую встроенную роль; существующую admin
// дополняет до полного каталога (union, идемпотентно), user не трогает —
// её права после bootstrap'а принадлежат данным.
func (uc *Seed) ensureRole(ctx context.Context, def domain.Role) error {
	stored, err := uc.roles.Get(ctx, def.Name)
	switch {
	case errors.Is(err, domain.ErrRoleNotFound):
		if err := uc.roles.Put(ctx, def); err != nil {
			return fmt.Errorf("seed role %q: %w", def.Name, err)
		}
		return nil
	case err != nil:
		return fmt.Errorf("get role %q: %w", def.Name, err)
	}
	if def.Name != domain.RoleAdmin {
		return nil
	}
	changed := false
	for _, p := range domain.AllPermissions() {
		if stored.Grant(p) {
			changed = true
		}
	}
	if !changed {
		return nil
	}
	if err := uc.roles.Put(ctx, stored); err != nil {
		return fmt.Errorf("update role %q: %w", stored.Name, err)
	}
	return nil
}

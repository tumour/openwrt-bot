// Package access — фича управления доступом: кто пользуется ботом (users)
// и что ему можно (roles → permissions). Approve-flow: незнакомец оставляет
// заявку (/start), носитель manage_users одобряет или отклоняет.
package access

import (
	"context"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// Порты фичи access. Оба стора — узкие агрегатные контракты (User и Role
// целиком), без generic-языка запросов: сегодня их реализует JSON-файл,
// завтра sqlite/postgres — контракты не меняются, совместимость нового
// адаптера доказывает общий контрактный сьют (accesstest).
//
// ВАЖНО: оба порта отдаёт ОДИН адаптер (sub-store'ами) под ОДНИМ локом —
// операция над users видит согласованное с roles состояние и наоборот.
// Гранулярность консистентности = одна операция порта; use cases пишут
// только после проверок, выполненных своими же вызовами портов.

// UserStore хранит пользователей. Put — upsert.
type UserStore interface {
	All(ctx context.Context) ([]domain.User, error)
	// Get возвращает domain.ErrUserNotFound, если пользователя нет.
	Get(ctx context.Context, id domain.UserID) (domain.User, error)
	Put(ctx context.Context, u domain.User) error
	// Delete возвращает domain.ErrUserNotFound, если пользователя нет.
	Delete(ctx context.Context, id domain.UserID) error
}

// RoleStore хранит роли. Put — upsert.
type RoleStore interface {
	All(ctx context.Context) ([]domain.Role, error)
	// Get возвращает domain.ErrRoleNotFound, если роли нет.
	Get(ctx context.Context, name domain.RoleName) (domain.Role, error)
	Put(ctx context.Context, r domain.Role) error
	// Delete возвращает domain.ErrRoleNotFound, если роли нет.
	Delete(ctx context.Context, name domain.RoleName) error
}

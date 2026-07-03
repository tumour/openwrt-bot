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
// Правило атомарности: use case, МЕНЯЮЩИЙ данные, работает только внутри
// Atomic.Update — все его проверки и записи выполняются как одна единица.
// Check-then-act напрямую через сторы — гонка: два админа одновременно
// удаляют друг друга, оба проходят guard «не последний» — бот без админа.
// Читающим use cases транзакция не нужна, они ходят в сторы напрямую.

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

// Atomic — unit of work фичи: fn выполняется атомарно относительно всех
// остальных мутаций хранилища доступа. Внутри fn пользоваться ТОЛЬКО
// переданными сторами (внешние ссылки на сторы того же адаптера — дедлок
// или грязное чтение, в зависимости от движка). JSON-адаптер держит лок на
// весь колбэк, SQL-адаптер откроет транзакцию и откатит её при ошибке fn.
type Atomic interface {
	Update(ctx context.Context, fn func(users UserStore, roles RoleStore) error) error
}

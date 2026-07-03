package domain

import "fmt"

// UserID — Telegram user ID. Value object: у пользователей Telegram ID строго
// положительный (отрицательные — группы/каналы, им доступ не выдаётся).
type UserID int64

// NewUserID валидирует Telegram user ID.
func NewUserID(id int64) (UserID, error) {
	if id <= 0 {
		return 0, fmt.Errorf("%w: %d", ErrInvalidUserID, id)
	}
	return UserID(id), nil
}

// UserStatus — этап жизненного цикла пользователя в approve-flow.
type UserStatus string

const (
	StatusPending UserStatus = "pending" // заявка отправлена, ждёт решения админа
	StatusActive  UserStatus = "active"  // допущен, команды бота доступны
)

// User — entity: пользователь бота. Инварианты держат конструкторы и методы:
// pending всегда без роли, active всегда с ролью, переход между статусами —
// только через Approve.
type User struct {
	ID     UserID
	Name   string   // display-имя из Telegram на момент заявки; для UI, не для идентификации
	Role   RoleName // пустая у pending
	Status UserStatus
}

// NewPendingUser — заявка на доступ (approve-flow): без роли до одобрения.
func NewPendingUser(id UserID, name string) User {
	return User{ID: id, Name: name, Status: StatusPending}
}

// NewActiveUser — сразу допущенный пользователь (env-сид админа при старте).
func NewActiveUser(id UserID, name string, role RoleName) User {
	return User{ID: id, Name: name, Role: role, Status: StatusActive}
}

// Approve переводит заявку в допущенного с ролью role. Одобрение уже
// активного — ErrNotPending: статусный переход существует один.
func (u *User) Approve(role RoleName) error {
	if u.Status != StatusPending {
		return fmt.Errorf("%w: user %d is %q", ErrNotPending, u.ID, u.Status)
	}
	u.Status = StatusActive
	u.Role = role
	return nil
}

// IsActive — допущен ли пользователь к командам бота.
func (u User) IsActive() bool { return u.Status == StatusActive }

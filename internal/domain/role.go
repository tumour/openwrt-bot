package domain

import (
	"fmt"
	"regexp"
)

// RoleName — value object. Формат намеренно узкий (латиница/цифры/подчёркивание,
// как у команд Telegram): имя показывается в кнопках и хранится в файлах,
// произвольные строки там не нужны.
type RoleName string

var roleNameRegexp = regexp.MustCompile(`^[a-z0-9_]{1,32}$`)

// Встроенные роли. Bootstrap создаёт их в пустом хранилище: env-админ получает
// RoleAdmin, одобренный через approve-flow — RoleUser. Дальше роли — данные:
// набор ролей и их права редактируются в рантайме.
const (
	RoleAdmin RoleName = "admin"
	RoleUser  RoleName = "user"
)

// NewRoleName валидирует имя роли.
func NewRoleName(s string) (RoleName, error) {
	if !roleNameRegexp.MatchString(s) {
		return "", fmt.Errorf("%w: %q", ErrInvalidRoleName, s)
	}
	return RoleName(s), nil
}

// String реализует fmt.Stringer.
func (n RoleName) String() string { return string(n) }

// Role — entity: именованный набор прав. Роли — данные (админ меняет их в
// рантайме), в отличие от каталога Permission. Связь роль→права — поле, а не
// отдельная сущность: как её кодировать (вложенный массив JSON, join-таблица
// SQL) — забота адаптера хранилища.
type Role struct {
	Name        RoleName
	Permissions []Permission
}

// NewRole валидирует имя и каждое право по каталогу; дубликаты прав схлопывает.
func NewRole(name string, permissions []string) (Role, error) {
	n, err := NewRoleName(name)
	if err != nil {
		return Role{}, err
	}
	seen := make(map[Permission]struct{}, len(permissions))
	perms := make([]Permission, 0, len(permissions))
	for _, raw := range permissions {
		p, err := NewPermission(raw)
		if err != nil {
			return Role{}, fmt.Errorf("role %q: %w", name, err)
		}
		if _, dup := seen[p]; dup {
			continue
		}
		seen[p] = struct{}{}
		perms = append(perms, p)
	}
	return Role{Name: n, Permissions: perms}, nil
}

// Has сообщает, есть ли у роли право.
func (r Role) Has(p Permission) bool {
	for _, have := range r.Permissions {
		if have == p {
			return true
		}
	}
	return false
}

// Grant добавляет роли право (идемпотентно). Возвращает true, если право
// действительно добавилось — вызывающий узнаёт, менялась ли роль.
func (r *Role) Grant(p Permission) bool {
	if r.Has(p) {
		return false
	}
	r.Permissions = append(r.Permissions, p)
	return true
}

// DefaultRoles — роли свежего бота: admin со всеми правами каталога, user —
// с базовыми (смотреть статус, список устройств, спидтест; управление
// устройствами и юзерами — только admin). Используются при инициализации
// пустого хранилища; дальше роли живут как данные.
func DefaultRoles() []Role {
	return []Role{
		{Name: RoleAdmin, Permissions: AllPermissions()},
		{Name: RoleUser, Permissions: []Permission{
			PermViewStatus, PermListDevices, PermRunSpeedtest,
		}},
	}
}

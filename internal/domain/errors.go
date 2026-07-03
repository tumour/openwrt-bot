package domain

import "errors"

// Доменные ошибки. Application и adapter слои оборачивают их через fmt.Errorf("...: %w", ErrXxx)
// и проверяют через errors.Is. Это позволяет верхним слоям отличать классы ошибок без
// разбора текстовых сообщений.
var (
	ErrInvalidMAC = errors.New("invalid MAC address")
	// ErrAlreadyInSet/ErrNotInSet — generic-ошибки членства MAC в nft-сете.
	// Сетов два (бан-лист, vpn-обход), семантику "что значит состоять в сете"
	// придаёт use case, а не ошибка.
	ErrAlreadyInSet = errors.New("mac already in set")
	ErrNotInSet     = errors.New("mac is not in set")
)

// Ошибки модели доступа (User/Role/Permission).
var (
	ErrInvalidUserID     = errors.New("invalid user id")
	ErrInvalidRoleName   = errors.New("invalid role name")
	ErrUnknownPermission = errors.New("unknown permission")
	ErrUserNotFound      = errors.New("user not found")
	ErrRoleNotFound      = errors.New("role not found")
	ErrNotPending        = errors.New("user is not pending")
	ErrNotActive         = errors.New("user is not active")
	// ErrForbidden — у инициатора операции нет требуемого права (или он не активен).
	ErrForbidden = errors.New("operation is not permitted")
	// ErrLastAdmin — операция оставила бы бота без единого активного носителя
	// PermManageUsers (удаление/разжалование последнего админа). Проверяется
	// в app-слое: это инвариант коллекции, а не одной сущности.
	ErrLastAdmin = errors.New("would leave no user able to manage users")
)

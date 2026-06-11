package domain

import "errors"

// Доменные ошибки. Application и adapter слои оборачивают их через fmt.Errorf("...: %w", ErrXxx)
// и проверяют через errors.Is. Это позволяет верхним слоям отличать классы ошибок без
// разбора текстовых сообщений.
var (
	ErrInvalidMAC    = errors.New("invalid MAC address")
	ErrAlreadyBanned = errors.New("device already banned")
	ErrNotBanned     = errors.New("device is not banned")
)

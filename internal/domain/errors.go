package domain

import "errors"

// Доменные ошибки. Application и adapter слои оборачивают их через fmt.Errorf("...: %w", ErrXxx)
// и проверяют через errors.Is. Это позволяет верхним слоям отличать классы ошибок без
// разбора текстовых сообщений.
var (
	ErrInvalidMAC  = errors.New("invalid MAC address")
	ErrInvalidRate = errors.New("invalid rate limit")
	// ErrAlreadyInSet/ErrNotInSet — generic-ошибки членства MAC в nft-сете.
	// Сетов два (бан-лист, vpn-обход), семантику "что значит состоять в сете"
	// придаёт use case, а не ошибка.
	ErrAlreadyInSet = errors.New("mac already in set")
	ErrNotInSet     = errors.New("mac is not in set")
)

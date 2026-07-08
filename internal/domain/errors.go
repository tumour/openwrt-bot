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
	// ErrInvalidAction/ErrInvalidMinutes — валидация входных данных таймера
	// (действие и задержка) на границе: битый payload кнопки или аргумент /timer.
	ErrInvalidAction  = errors.New("invalid action")
	ErrInvalidMinutes = errors.New("invalid delay in minutes")
	// ErrTaskNotFound — отмена несуществующего таймера. Штатно: кнопка отмены из
	// сообщения, пережившего сам таймер (сработал/отменён/рестарт бота). Адаптер
	// планировщика транслирует сюда инфраструктурную ошибку движка — как nftables
	// транслирует stderr в ErrNotInSet.
	ErrTaskNotFound = errors.New("scheduled task not found")
)

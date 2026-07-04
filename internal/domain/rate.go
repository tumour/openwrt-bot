package domain

import (
	"fmt"
	"strconv"
)

// Rate — value object: лимит скорости в КБайт/с (КБ = 1024 байта, как kbytes
// в nftables). Значение, созданное через NewRate, гарантированно валидно.
// Zero value (0) легально существует только как «лимит не задан» в DTO верхних
// слоёв (аналог time.Time.IsZero) — NewRate ноль никогда не выдаёт.
type Rate int

// maxRateKBps — верхняя граница лимита: ~1 ГБ/с, заведомо выше любого домашнего
// канала. Отсекает бессмыслицу и переполнения в расчётах на стороне nft.
const maxRateKBps = 1_000_000

// NewRate валидирует лимит скорости в КБ/с: целое 1..1_000_000.
// «Нет лимита» выражается отсутствием Rate (снятие — отдельная операция),
// поэтому ноль невалиден.
func NewRate(kbps int) (Rate, error) {
	if kbps < 1 || kbps > maxRateKBps {
		return 0, fmt.Errorf("%w: %d", ErrInvalidRate, kbps)
	}
	return Rate(kbps), nil
}

// KBps — численное значение в КБ/с для форматирования и расчётов.
func (r Rate) KBps() int { return int(r) }

// IsZero — «лимит не задан» (см. комментарий к типу).
func (r Rate) IsZero() bool { return r == 0 }

// String реализует fmt.Stringer: «512».
func (r Rate) String() string { return strconv.Itoa(int(r)) }

package domain

import (
	"fmt"
	"regexp"
	"strings"
)

// MAC — value object. Если значение типа MAC существует, оно гарантированно
// валидно (создаётся только через NewMAC). Это убирает из бизнес-кода
// необходимость перепроверять формат.
type MAC string

var macRegexp = regexp.MustCompile(`^[0-9a-f]{2}(:[0-9a-f]{2}){5}$`)

// NewMAC валидирует и нормализует MAC-адрес. Принимает регистр-независимо
// и разделители `:` или `-`, на выходе всегда lowercase с `:`.
func NewMAC(s string) (MAC, error) {
	normalized := strings.ToLower(strings.ReplaceAll(s, "-", ":"))
	if !macRegexp.MatchString(normalized) {
		return "", fmt.Errorf("%w: %q", ErrInvalidMAC, s)
	}
	return MAC(normalized), nil
}

// String реализует fmt.Stringer.
func (m MAC) String() string { return string(m) }

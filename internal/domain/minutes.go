package domain

import (
	"fmt"
	"time"
)

// Minutes — value object: задержка таймера в минутах. Если значение существует,
// оно в допустимом диапазоне (создаётся только через NewMinutes), и верхним слоям
// незачем перепроверять — как у MAC и Rate. Верхняя граница — сутки: таймеры
// короткоживущие, «бан на неделю» — это ручной бан, а не таймер.
type Minutes int

const (
	minMinutes = 1
	maxMinutes = 24 * 60 // сутки
)

// NewMinutes валидирует задержку. Вне диапазона [1..1440] → ErrInvalidMinutes.
func NewMinutes(n int) (Minutes, error) {
	if n < minMinutes || n > maxMinutes {
		return 0, fmt.Errorf("%w: %d (допустимо %d..%d)", ErrInvalidMinutes, n, minMinutes, maxMinutes)
	}
	return Minutes(n), nil
}

// Duration переводит минуты в time.Duration — для арифметики со временем срабатывания.
func (m Minutes) Duration() time.Duration { return time.Duration(m) * time.Minute }

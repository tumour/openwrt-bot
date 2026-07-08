package system

import "time"

// Clock — реальные системные часы: реализация scheduler.Clock. Удовлетворяет
// порт структурно (без импорта scheduler): AfterFunc возвращает метод-значение
// time.Timer.Stop как stop-функцию, поэтому все сигнатуры — только stdlib.
// Единственный шов между планировщиком и реальным временем — так таймеры
// становятся тестируемыми фейк-часами.
type Clock struct{}

func NewClock() Clock { return Clock{} }

// Now — текущее время.
func (Clock) Now() time.Time { return time.Now() }

// AfterFunc вызывает fire по истечении d в отдельной горутине (семантика
// time.AfterFunc); возвращаемый stop отменяет ещё не сработавший таймер.
func (Clock) AfterFunc(d time.Duration, fire func()) (stop func() bool) {
	return time.AfterFunc(d, fire).Stop
}

package telegram

import "time"

// backoff — экспоненциальная задержка между повторами: min, min*2, ... cap max.
// Используется connect-фазой Run и поллером — обоим нужен один и тот же ритм
// «не долбить лежащий socks, но быстро вернуться, когда сеть оживёт».
// НЕ потокобезопасен: каждый потребитель держит свой экземпляр в одной горутине.
type backoff struct {
	min, max time.Duration
	cur      time.Duration // 0 = следующий next() вернёт min
}

// newBackoff — параметры подключения к Telegram: 5s→10→20→40→60, дальше по 60.
// Константы кода, не конфиг: тюнить их по месту незачем, а тесты инжектят
// малые значения прямо в поля структуры.
func newBackoff() backoff {
	return backoff{min: 5 * time.Second, max: 60 * time.Second}
}

// next возвращает следующую задержку и удваивает текущую вплоть до max.
func (b *backoff) next() time.Duration {
	if b.cur == 0 {
		b.cur = b.min
	} else {
		b.cur *= 2
		if b.cur > b.max {
			b.cur = b.max
		}
	}
	return b.cur
}

// reset возвращает backoff в исходное состояние — зовётся после успешного вызова.
func (b *backoff) reset() { b.cur = 0 }

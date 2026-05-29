// Package middleware содержит сквозные обёртки для всех handler'ов.
// Применяются глобально через bot.Use(...).
package middleware

import (
	"log/slog"

	tele "gopkg.in/telebot.v3"
)

// Auth пропускает только сообщения от chat_id из whitelist'а. Неавторизованные
// игнорируются молча (не отвечаем — не палим существование бота сканерам).
func Auth(allowed []int64, logger *slog.Logger) tele.MiddlewareFunc {
	set := make(map[int64]struct{}, len(allowed))
	for _, id := range allowed {
		set[id] = struct{}{}
	}
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			id := c.Sender().ID
			if _, ok := set[id]; !ok {
				logger.Warn("unauthorized",
					"chat_id", id,
					"username", c.Sender().Username,
					"text", c.Text(),
				)
				return nil
			}
			return next(c)
		}
	}
}

// Package middleware содержит сквозные обёртки для всех handler'ов.
// Применяются глобально через bot.Use(...).
package middleware

import (
	"log/slog"

	tele "gopkg.in/telebot.v3"
)

// Auth пропускает только сообщения от user ID из whitelist'а (отправитель,
// не chat — в группах они различаются). Неавторизованные игнорируются молча
// (не отвечаем — не палим существование бота сканерам).
func Auth(allowedUserIDs []int64, logger *slog.Logger) tele.MiddlewareFunc {
	set := make(map[int64]struct{}, len(allowedUserIDs))
	for _, id := range allowedUserIDs {
		set[id] = struct{}{}
	}
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			sender := c.Sender()
			// У части апдейтов (channel post и т.п.) отправителя нет —
			// такие точно не от whitelist-юзера, отбрасываем без паники.
			if sender == nil {
				return nil
			}
			if _, ok := set[sender.ID]; !ok {
				logger.Warn("unauthorized",
					"user_id", sender.ID,
					"username", sender.Username,
					"text", c.Text(),
				)
				return nil
			}
			return next(c)
		}
	}
}

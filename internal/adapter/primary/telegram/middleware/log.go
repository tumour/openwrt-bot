package middleware

import (
	"log/slog"
	"time"

	tele "gopkg.in/telebot.v3"
)

// Log пишет каждое обработанное обращение (после Auth, поэтому только от своих).
// Полезно видеть, что команды доходят и сколько они занимают.
func Log(logger *slog.Logger) tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			start := time.Now()
			err := next(c)
			logger.Info("command",
				"from", c.Sender().Username,
				"text", c.Text(),
				"duration", time.Since(start),
				"err", err,
			)
			return err
		}
	}
}

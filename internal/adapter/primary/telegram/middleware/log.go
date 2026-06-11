package middleware

import (
	"log/slog"
	"time"

	tele "gopkg.in/telebot.v3"
)

// Log пишет каждое обработанное обращение (после Auth, поэтому только от своих)
// и служит error boundary: полную цепочку ошибки handler'а логирует и гасит
// (юзеру короткое сообщение уже отправил сам handler — см. контракт в package
// doc handler'а). Не возвращаем err наверх, чтобы telebot.OnError не дублировал
// лог неструктурированной строкой.
func Log(logger *slog.Logger) tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			start := time.Now()
			err := next(c)
			if err != nil {
				logger.Error("command failed",
					"from", c.Sender().Username,
					"text", c.Text(),
					"duration", time.Since(start),
					"err", err,
				)
				return nil
			}
			logger.Info("command",
				"from", c.Sender().Username,
				"text", c.Text(),
				"duration", time.Since(start),
			)
			return nil
		}
	}
}

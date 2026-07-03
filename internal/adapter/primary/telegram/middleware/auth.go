// Package middleware содержит сквозные обёртки для всех handler'ов.
// Применяются глобально через bot.Use(...).
package middleware

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/tumour/openwrt-bot/internal/app/access"
	tele "gopkg.in/telebot.v3"
)

// authTimeout — лимит на поход в хранилище доступа при проверке отправителя.
const authTimeout = 5 * time.Second

// AccessChecker — «допущен ли отправитель и что ему можно». Интерфейс объявлен
// у потребителя (реализация — access.Check), чтобы тестировать Auth фейком.
type AccessChecker interface {
	Execute(ctx context.Context, in access.CheckInput) (access.CheckOutput, error)
}

// Auth решает судьбу каждого апдейта по отправителю (не chat — в группах они
// различаются): допущенному кладёт Grant в контекст и пропускает дальше;
// незнакомцу с /start даёт подать заявку (onRequest — approve-flow);
// всё остальное от чужих молча отбрасывается (не отвечаем — не палим
// существование бота сканерам). Ошибка проверки = отказ (fail closed).
func Auth(check AccessChecker, onRequest tele.HandlerFunc, logger *slog.Logger) tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			sender := c.Sender()
			// У части апдейтов (channel post и т.п.) отправителя нет —
			// такие точно не от пользователя, отбрасываем без паники.
			if sender == nil {
				return nil
			}
			ctx, cancel := context.WithTimeout(BaseContext(c), authTimeout)
			defer cancel()

			out, err := check.Execute(ctx, access.CheckInput{UserID: sender.ID})
			if err != nil {
				logger.Error("auth: проверка доступа упала", "user_id", sender.ID, "err", err)
				return nil
			}
			if out.Allowed {
				PutGrant(c, out.Grant)
				return next(c)
			}
			if isStartMessage(c) && onRequest != nil {
				return onRequest(c)
			}
			logger.Warn("unauthorized",
				"user_id", sender.ID,
				"username", sender.Username,
				"text", c.Text(),
			)
			return nil
		}
	}
}

// isStartMessage — текстовое сообщение /start (не callback). Только оно
// открывает незнакомцу approve-flow.
func isStartMessage(c tele.Context) bool {
	return c.Callback() == nil && strings.HasPrefix(strings.TrimSpace(c.Text()), "/start")
}

package middleware

import (
	"context"

	tele "gopkg.in/telebot.v3"
)

// Ключ, под которым в telebot.Context лежит базовый context приложения.
// Приватный: снаружи только PutBaseContext/BaseContext.
const baseCtxKey = "base_ctx"

// PutBaseContext кладёт базовый context приложения в telebot.Context апдейта.
// Вызывает бот (track-middleware) на каждом апдейте: handlers строят свои
// таймауты поверх этого контекста, и shutdown (SIGTERM) отменяет их все разом —
// вместе с запущенными exec'ами. До этого handlers жили на context.Background()
// и graceful shutdown до них не доходил.
func PutBaseContext(c tele.Context, ctx context.Context) {
	if ctx != nil {
		c.Set(baseCtxKey, ctx)
	}
}

// BaseContext возвращает базовый context приложения, положенный PutBaseContext.
// Если его нет (юнит-тест, прямой вызов handler'а) — context.Background():
// handler обязан работать и без обвязки бота.
func BaseContext(c tele.Context) context.Context {
	if ctx, ok := c.Get(baseCtxKey).(context.Context); ok && ctx != nil {
		return ctx
	}
	return context.Background()
}

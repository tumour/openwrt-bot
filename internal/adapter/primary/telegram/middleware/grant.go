package middleware

import (
	"github.com/tumour/openwrt-bot/internal/app/access"
	tele "gopkg.in/telebot.v3"
)

// Ключ, под которым в telebot.Context лежит Grant допущенного отправителя.
// Приватный: снаружи только PutGrant/GrantFrom.
const grantKey = "grant"

// PutGrant кладёт «пропуск» отправителя в контекст апдейта. Вызывает Auth
// после успешной проверки; всё, что ниже по цепочке (guard'ы, handlers,
// клавиатуры), решает по нему, что выполнять и что рисовать.
func PutGrant(c tele.Context, g access.Grant) {
	c.Set(grantKey, g)
}

// GrantFrom возвращает Grant, положенный Auth. ok=false — апдейт прошёл вне
// Auth (юнит-тест, прямой вызов): вызывающий обязан отказать (fail closed).
func GrantFrom(c tele.Context) (access.Grant, bool) {
	g, ok := c.Get(grantKey).(access.Grant)
	return g, ok
}

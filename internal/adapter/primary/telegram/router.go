package telegram

import (
	"github.com/tumour/openwrt-bot/internal/adapter/primary/telegram/handler"
	tele "gopkg.in/telebot.v3"
)

// Handlers собирает все handlers, нужные боту. Передаются снаружи (composition
// root) — Bot их не создаёт сам. Группировка в struct даёт compile-time гарантию:
// добавил поле — забыл выставить = nil pointer dereference на старте, видно сразу.
type Handlers struct {
	Status *handler.Status
	Ban    *handler.Ban
	Unban  *handler.Unban
	List   *handler.List
}

// registerRoutes маппит команды Telegram на методы handler'ов.
// Единственное место, где меняем при добавлении новой команды.
func registerRoutes(bot *tele.Bot, h Handlers) {
	bot.Handle("/status", h.Status.Handle)
	bot.Handle("/start", h.Status.Handle) // алиас для удобства первого запуска
	bot.Handle("/ban", h.Ban.Handle)
	bot.Handle("/unban", h.Unban.Handle)
	bot.Handle("/list", h.List.Handle)
}

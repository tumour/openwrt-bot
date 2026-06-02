package telegram

import (
	"github.com/tumour/openwrt-bot/internal/adapter/primary/telegram/handler"
	tele "gopkg.in/telebot.v3"
)

// Handlers собирает все handlers, нужные боту. Передаются снаружи (composition
// root) — Bot их не создаёт сам. Группировка в struct даёт compile-time гарантию:
// добавил поле — забыл выставить = nil pointer dereference на старте, видно сразу.
type Handlers struct {
	Status    *handler.Status
	Ban       *handler.Ban
	Unban     *handler.Unban
	List      *handler.List
	SpeedTest *handler.SpeedTest
}

// command — описание одной команды в ОДНОМ месте: и маршрут (handler), и пункт
// нативного меню Telegram. Единый источник, чтобы список не разъезжался между
// bot.Handle и SetCommands.
type command struct {
	name   string           // без ведущего "/" — формат Telegram BotCommand (lowercase)
	desc   string           // описание для меню (≡ / автодополнение по «/»), 3-256 симв.
	handle tele.HandlerFunc // что вызывать
	inMenu bool             // показывать в меню (алиасы вроде /start — false)
}

// commands — все команды бота. Порядок = порядок в меню Telegram.
// Единственное место, где меняем при добавлении команды: одна строка — и маршрут,
// и пункт меню подхватятся автоматически.
func commands(h Handlers) []command {
	return []command{
		{"status", "Статус роутера: uptime, нагрузка, память, температура", h.Status.Handle, true},
		{"list", "Устройства в сети и баны", h.List.Handle, true},
		{"speedtest", "Замер скорости интернет-канала", h.SpeedTest.Handle, true},
		{"ban", "Забанить устройство: /ban AA:BB:CC:DD:EE:FF", h.Ban.Handle, true},
		{"unban", "Разбанить устройство: /unban AA:BB:CC:DD:EE:FF", h.Unban.Handle, true},
		{"start", "Запуск и статус роутера", h.Status.Handle, false}, // алиас /status, в меню не нужен
	}
}

// registerRoutes маппит команды Telegram на handler'ы.
func registerRoutes(bot *tele.Bot, h Handlers) {
	for _, c := range commands(h) {
		bot.Handle("/"+c.name, c.handle)
	}
}

// menuCommands — пункты для нативного меню Telegram (кнопка ≡ + автодополнение).
// Отбирает из commands только помеченные inMenu (без алиасов).
func menuCommands(h Handlers) []tele.Command {
	cmds := commands(h)
	menu := make([]tele.Command, 0, len(cmds))
	for _, c := range cmds {
		if c.inMenu {
			menu = append(menu, tele.Command{Text: c.name, Description: c.desc})
		}
	}
	return menu
}

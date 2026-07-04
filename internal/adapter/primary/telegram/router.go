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
	Devices   *handler.Devices // /list + callback-кнопки карточек
	SpeedTest *handler.SpeedTest
	VPNOff    *handler.VPNOff
	VPNOn     *handler.VPNOn
	Limit     *handler.Limit
	Unlimit   *handler.Unlimit
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
//
// MAC-команды (ban/unban/vpnoff/vpnon) из меню убраны (inMenu:false): тот же
// функционал доступен кнопками в карточках /list, а руками их набирают редко.
// Сами команды работают, если ввести текстом.
func commands(h Handlers) []command {
	return []command{
		{"list", "Устройства в сети: бан и VPN по кнопкам", h.Devices.HandleList, true},
		{"status", "Статус роутера: uptime, нагрузка, память, температура", h.Status.Handle, true},
		{"speedtest", "Замер скорости интернет-канала", h.SpeedTest.Handle, true},
		{"ban", "Забанить устройство: /ban AA:BB:CC:DD:EE:FF", h.Ban.Handle, false},
		{"unban", "Разбанить устройство: /unban AA:BB:CC:DD:EE:FF", h.Unban.Handle, false},
		{"vpnoff", "Пустить устройство мимо VPN: /vpnoff AA:BB:CC:DD:EE:FF", h.VPNOff.Handle, false},
		{"vpnon", "Вернуть устройство в VPN: /vpnon AA:BB:CC:DD:EE:FF", h.VPNOn.Handle, false},
		{"limit", "Ограничить скорость: /limit AA:BB:CC:DD:EE:FF 512 (КБ/с)", h.Limit.Handle, false},
		{"unlimit", "Снять лимит скорости: /unlimit AA:BB:CC:DD:EE:FF", h.Unlimit.Handle, false},
		{"start", "Привет + клавиатура управления", startHandler(h), false},
	}
}

// Кнопки постоянной reply-клавиатуры (живёт под полем ввода, видна всегда —
// в отличие от нативного меню, зарытого в «≡»). Текст кнопки = endpoint:
// telebot матчит входящее сообщение по точному тексту.
var (
	btnDevices   = tele.Btn{Text: "📱 Устройства"}
	btnStatus    = tele.Btn{Text: "📊 Статус"}
	btnSpeedTest = tele.Btn{Text: "🚀 Спидтест"}
)

// mainKeyboard — постоянная клавиатура управления. ResizeKeyboard — компактные
// кнопки по высоте текста, OneTimeKeyboard НЕ ставим: клавиатура должна жить.
func mainKeyboard() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{ResizeKeyboard: true}
	m.Reply(
		m.Row(btnDevices),
		m.Row(btnStatus, btnSpeedTest),
	)
	return m
}

// startHandler — /start: приветствие + установка постоянной клавиатуры.
func startHandler(h Handlers) tele.HandlerFunc {
	return func(c tele.Context) error {
		return c.Send(
			"Роутер на связи 🛜\n\n"+
				"Управление — кнопками внизу. Бан и VPN per-device — в карточках устройств (📱 Устройства).",
			mainKeyboard())
	}
}

// registerRoutes маппит команды Telegram на handler'ы, вешает кнопки
// постоянной клавиатуры и callback'и inline-кнопок (карточки устройств).
func registerRoutes(bot *tele.Bot, h Handlers) {
	for _, c := range commands(h) {
		bot.Handle("/"+c.name, c.handle)
	}
	bot.Handle(&btnDevices, h.Devices.HandleList)
	bot.Handle(&btnStatus, h.Status.Handle)
	bot.Handle(&btnSpeedTest, h.SpeedTest.Handle)
	h.Devices.RegisterCallbacks(bot)
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

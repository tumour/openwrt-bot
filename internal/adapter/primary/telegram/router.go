package telegram

import (
	"github.com/tumour/openwrt-bot/internal/adapter/primary/telegram/handler"
	"github.com/tumour/openwrt-bot/internal/adapter/primary/telegram/middleware"
	"github.com/tumour/openwrt-bot/internal/app/access"
	"github.com/tumour/openwrt-bot/internal/domain"
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
	Access    *handler.Access // approve-flow + /users
}

// command — описание одной команды в ОДНОМ месте: маршрут (handler), пункт
// нативного меню Telegram, кнопка постоянной клавиатуры и требуемое право.
// Единый источник: маршруты, меню, клавиатура и проверка прав не разъезжаются.
type command struct {
	name   string            // без ведущего "/" — формат Telegram BotCommand (lowercase)
	desc   string            // описание для меню (≡ / автодополнение по «/»), 3-256 симв.
	handle tele.HandlerFunc  // что вызывать
	inMenu bool              // показывать в меню (алиасы вроде /start — false)
	perm   domain.Permission // требуемое право; "" — доступно любому допущенному.
	// Право работает на двух уровнях: guard не пустит выполнение, а меню и
	// клавиатура без права не покажут пункт («нет пермишена = нет кнопки»).
	btn *tele.Btn // кнопка постоянной reply-клавиатуры (nil — нет кнопки)
}

// Кнопки постоянной reply-клавиатуры (живёт под полем ввода, видна всегда —
// в отличие от нативного меню, зарытого в «≡»). Текст кнопки = endpoint:
// telebot матчит входящее сообщение по точному тексту.
var (
	btnDevices   = tele.Btn{Text: "📱 Устройства"}
	btnStatus    = tele.Btn{Text: "📊 Статус"}
	btnSpeedTest = tele.Btn{Text: "🚀 Спидтест"}
	btnUsers     = tele.Btn{Text: "👥 Доступ"}
)

// commands — все команды бота. Порядок = порядок в меню Telegram.
// Единственное место, где меняем при добавлении команды: одна строка — и
// маршрут, и меню, и клавиатура, и право подхватятся автоматически.
//
// MAC-команды (ban/unban/vpnoff/vpnon) из меню убраны (inMenu:false): тот же
// функционал доступен кнопками в карточках /list, а руками их набирают редко.
// Сами команды работают, если ввести текстом — под тем же правом.
func commands(h Handlers) []command {
	return []command{
		{"list", "Устройства в сети: бан и VPN по кнопкам", h.Devices.HandleList, true, domain.PermListDevices, &btnDevices},
		{"status", "Статус роутера: uptime, нагрузка, память, температура", h.Status.Handle, true, domain.PermViewStatus, &btnStatus},
		{"speedtest", "Замер скорости интернет-канала", h.SpeedTest.Handle, true, domain.PermRunSpeedtest, &btnSpeedTest},
		{"users", "Пользователи: заявки, роли, удаление", h.Access.HandleUsers, true, domain.PermManageUsers, &btnUsers},
		{"ban", "Забанить устройство: /ban AA:BB:CC:DD:EE:FF", h.Ban.Handle, false, domain.PermBanDevices, nil},
		{"unban", "Разбанить устройство: /unban AA:BB:CC:DD:EE:FF", h.Unban.Handle, false, domain.PermBanDevices, nil},
		{"vpnoff", "Пустить устройство мимо VPN: /vpnoff AA:BB:CC:DD:EE:FF", h.VPNOff.Handle, false, domain.PermManageVPN, nil},
		{"vpnon", "Вернуть устройство в VPN: /vpnon AA:BB:CC:DD:EE:FF", h.VPNOn.Handle, false, domain.PermManageVPN, nil},
		{"start", "Клавиатура управления / заявка на доступ", startHandler(h), false, "", nil},
	}
}

// guard пропускает handler только при наличии права. Grant кладёт Auth; его
// отсутствие — вызов вне цепочки (не должно случиться) → fail closed.
// Прав нет — сообщению вежливый отказ (кнопку юзер не видел, значит набрал
// команду руками), callback'у — toast.
func guard(perm domain.Permission, next tele.HandlerFunc) tele.HandlerFunc {
	return func(c tele.Context) error {
		g, ok := middleware.GrantFrom(c)
		if !ok {
			return nil
		}
		if perm != "" && !g.Has(perm) {
			if c.Callback() != nil {
				return c.Respond(&tele.CallbackResponse{Text: "⛔ Нет прав"})
			}
			return c.Send("⛔ Недостаточно прав.")
		}
		return next(c)
	}
}

// mainKeyboard — постоянная клавиатура по правам: нет права = нет кнопки.
// ResizeKeyboard — компактные кнопки, OneTimeKeyboard НЕ ставим: живёт всегда.
func mainKeyboard(h Handlers, g access.Grant) *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{ResizeKeyboard: true}
	var btns []tele.Btn
	for _, cmd := range commands(h) {
		if cmd.btn != nil && g.Has(cmd.perm) {
			btns = append(btns, *cmd.btn)
		}
	}
	// По две кнопки в ряд — на телефоне читаемо при любом составе прав.
	var rows []tele.Row
	for len(btns) > 0 {
		n := min(2, len(btns))
		rows = append(rows, tele.Row(btns[:n]))
		btns = btns[n:]
	}
	m.Reply(rows...)
	return m
}

// startHandler — /start от допущенного: приветствие + клавиатура по правам +
// личное меню команд. (/start незнакомца до сюда не доходит — Auth уводит
// его в approve-flow.)
func startHandler(h Handlers) tele.HandlerFunc {
	return func(c tele.Context) error {
		g, ok := middleware.GrantFrom(c)
		if !ok {
			return nil
		}
		// Личное меню «≡» по правам (scoped на этот чат). Best-effort:
		// без меню бот работает, ошибку глотаем сознательно.
		_ = c.Bot().SetCommands(menuCommandsFor(h, g), tele.CommandScope{
			Type: tele.CommandScopeChat, ChatID: int64(g.User.ID),
		})
		return c.Send(
			"Роутер на связи 🛜\n\n"+
				"Управление — кнопками внизу. Бан и VPN per-device — в карточках устройств (📱 Устройства).",
			mainKeyboard(h, g))
	}
}

// registerRoutes маппит команды на handler'ы (каждый под guard'ом своего
// права), вешает кнопки постоянной клавиатуры и callback'и inline-кнопок.
func registerRoutes(bot *tele.Bot, h Handlers) {
	for _, cmd := range commands(h) {
		bot.Handle("/"+cmd.name, guard(cmd.perm, cmd.handle))
		if cmd.btn != nil {
			bot.Handle(cmd.btn, guard(cmd.perm, cmd.handle))
		}
	}
	h.Devices.RegisterCallbacks(bot, guard)
	h.Access.RegisterCallbacks(bot, guard)
}

// menuCommandsFor — пункты меню «≡» для конкретного пользователя: inMenu
// И право на команду. Право "" не фильтрует.
func menuCommandsFor(h Handlers, g access.Grant) []tele.Command {
	cmds := commands(h)
	menu := make([]tele.Command, 0, len(cmds))
	for _, c := range cmds {
		if c.inMenu && (c.perm == "" || g.Has(c.perm)) {
			menu = append(menu, tele.Command{Text: c.name, Description: c.desc})
		}
	}
	return menu
}

// defaultMenuCommands — глобальное меню (для чатов без личного): только
// /start. Незнакомец не должен видеть команд, о правах которых бот ещё
// ничего не решил; допущенный получит личное меню первым же /start.
func defaultMenuCommands() []tele.Command {
	return []tele.Command{{Text: "start", Description: "Клавиатура управления / заявка на доступ"}}
}

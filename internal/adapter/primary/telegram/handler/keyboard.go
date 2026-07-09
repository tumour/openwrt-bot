package handler

import tele "gopkg.in/telebot.v3"

// Постоянная reply-клавиатура. Живёт в handler (а не в router): раскладка —
// часть UI-ответа, и хендлеры прикрепляют её к текстовым ответам (см. answer);
// router лишь вешает кнопки на маршруты. Текст кнопки = endpoint: telebot
// матчит входящее сообщение по точному тексту.
var (
	BtnDevices   = tele.Btn{Text: "📱 Устройства"}
	BtnStatus    = tele.Btn{Text: "📊 Статус"}
	BtnSpeedTest = tele.Btn{Text: "🚀 Спидтест"}
	BtnTimers    = tele.Btn{Text: "⏰ Таймеры"}
)

// MainKeyboard — постоянная клавиатура управления. ResizeKeyboard — компактные
// кнопки по высоте текста, OneTimeKeyboard НЕ ставим: клавиатура должна жить.
func MainKeyboard() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{ResizeKeyboard: true}
	m.Reply(
		m.Row(BtnDevices),
		m.Row(BtnStatus, BtnSpeedTest),
		m.Row(BtnTimers),
	)
	return m
}

// answer — текстовый ответ с прикреплённой постоянной клавиатурой. Reply-панель
// в Telegram — приложение к сообщению, а не свойство чата: клиент её теряет
// (перелогин, чистка кэша, новая сессия), и вернуть может только новое сообщение
// с markup. Прикрепляя клавиатуру к каждому текстовому ответу, чиним «кнопки
// пропали — жми /start». Не для ответов с inline-кнопками (/list, /timers):
// одно сообщение несёт один markup, а inline-ответы reply-панель и не сбивают.
func answer(c tele.Context, text string, opts ...interface{}) error {
	return c.Send(text, append(opts, MainKeyboard())...)
}

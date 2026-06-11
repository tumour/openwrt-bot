// Package handler — driving adapter: каждая команда Telegram превращается в
// вызов соответствующего use case. Handler НЕ содержит бизнес-логики, только
// I/O-преобразование: telebot.Context → use case Input → presenter → Send.
//
// Контракт ошибок: при ошибке use case handler шлёт юзеру короткое сообщение
// без технических деталей и возвращает полную ошибку наверх — её логирует
// middleware.Log (внутренности exec/stderr не должны утекать в чат).
package handler

import "time"

// handlerTimeout — внутренний лимит на вызов use case, чтобы зависший exec
// не повесил бот. 10 секунд с запасом для самого медленного `ubus call` на
// слабом роутере. Исключение — /speedtest, у него свой speedtestTimeout.
const handlerTimeout = 10 * time.Second

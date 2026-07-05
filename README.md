# openwrt-bot (skeleton)

Telegram-бот для управления домашним роутером (OpenWrt на Xiaomi AX3200).
Эталонный skeleton — демонстрация Hexagonal Architecture на Go.

## Архитектура

Hexagonal / Ports & Adapters. Бизнес-логика в центре, всё внешнее (Telegram,
nftables, ubus) — снаружи, подключается через интерфейсы.

```
                ┌────────────────────────────────┐
                │  domain  ─ pure Go, no imports │
                └─────────────┬──────────────────┘
                              │ used by
                ┌─────────────▼──────────────────┐
                │  app  ─ use cases + ports      │
                └──────┬──────────────────┬──────┘
                       │ implemented by   │ called by
        ┌──────────────▼───┐      ┌───────▼──────────────┐
        │ adapter/secondary│      │ adapter/primary       │
        │ (driven)         │      │ (driving)             │
        │ nftables, ubus   │      │ telegram bot, http api│
        └──────────────────┘      └───────────────────────┘

                composition root: cmd/bot/main.go
```

### Dependency rule

Стрелки **только внутрь**:

- `domain` — ничего не импортирует, кроме stdlib.
- `app` — импортирует `domain`. Не знает ни о Telegram, ни о nftables.
- `adapter/*` — импортирует `app` + `domain`. Реализует порты из `app`.
- `cmd/bot/main.go` — единственное место, где импортируется всё. Composition root.

Нарушение dependency rule = архитектура сломана. Проверяется глазами на code-review
и опционально через `go-arch-lint` или ручной grep в CI.

## Папки

| Папка | Назначение |
|---|---|
| `cmd/bot/` | Entrypoint. Только wiring зависимостей. Никакой логики. |
| `internal/domain/` | Entities, Value Objects, доменные ошибки. Pure Go. |
| `internal/app/<feature>/` | Use cases, сгруппированные по фичам. **Каждая фича** — отдельная папка (`device/`, `status/`, ...) с собственными портами (`ports.go`) и use cases. Не плоский `app/`. |
| `internal/adapter/primary/telegram/` | Driving adapter: принимает входы из Telegram, вызывает use cases. |
| `internal/adapter/primary/httpapi/` | Driving adapter №2: локальный HTTP API для LuCI-панели — **те же use cases**, другой вход. Primary-адаптеры друг о друге не знают. |
| `internal/adapter/secondary/{nftables,ubus,system}/` | Driven adapters: реализации портов через exec. |
| `internal/platform/` | Cross-cutting infrastructure: config, logger, graceful shutdown. |
| `luci-app-openwrt-bot/` | LuCI-панель (rpcd ucode-плагин + JS-вьюха). Раскладка LuCI-feed (`htdocs/`+`root/`) — выносится в отдельный пакет без перекладывания файлов. Клиент HTTP API, Go-код о ней не знает. |

### Почему `app/` структурирован по фичам, а не плоский

Плоский `app/` с десятками `*.go` плохо масштабируется: непонятно, где границы, и порты приходится держать в общем `ports.go`, который растёт неограниченно.

**Vertical slicing по фиче** (`app/device/`, `app/status/`, ...) даёт:
- Каждая фича владеет своими портами рядом — реальная реализация принципа *«define interfaces at point of use»*.
- 30 use cases растут *вширь* по папкам (5 фич × 6 use cases), а не в одну плоскую кучу.
- Имена use cases короче за счёт контекста package: `device.Ban` вместо `app.BanDevice`.
- Удалить фичу = удалить папку, остальные не зацепятся.

## Как добавить use case (пошагово)

Пример: «добавить speedtest WAN».

1. **Решить, в какую фичу** ложится — `app/network/` (новая папка) или существующая.
2. **Domain** (если нужны новые сущности): например `internal/domain/bandwidth.go` — value object `Bandwidth` с валидацией.
3. **Port** (если нужен новый источник данных): добавить в `internal/app/network/ports.go` интерфейс `SpeedTestPort`.
4. **Use case**: `internal/app/network/speedtest.go` — struct `SpeedTest` + `Execute`. Тест `speedtest_test.go` с моками портов.
5. **Adapter** (если порт новый): реализация в `internal/adapter/secondary/<name>/`. Тест с моком `system.Runner`.
6. **Handler**: `internal/adapter/primary/telegram/handler/speedtest.go`. Вызывает `network.SpeedTest.Execute`, передаёт результат в presenter.
7. **Presenter**: `internal/adapter/primary/telegram/presenter/speedtest.go` — `Format(network.SpeedTestOutput) string`.
8. **Router**: зарегистрировать `/speedtest → handler` в `internal/adapter/primary/telegram/router.go`.
9. **Wire** в `cmd/bot/main.go` — собрать use case и handler через конструкторы.

Каждый шаг — отдельный коммит. Каждый файл — небольшой и сфокусированный.

## Граница domain ↔ app

Частая путаница: «вся бизнес-логика в app/». На самом деле:

- **`domain/`** — правила и инварианты, верные независимо от приложения. Пример: «MAC всегда в формате xx:xx:xx:xx:xx:xx», «`00:00:00:00:00:00` — broadcast, нельзя банить».
- **`app/`** — workflow и orchestration портов. Пример: «повторный бан = no-op», «сначала спросить DHCP, потом nftables, потом склеить».

Критерий: если завтра заменить Telegram на REST и nftables на iptables — что не должно поменяться? Всё неизменное = `domain` + `app`. Меняется только `adapter/`.

## Принципы

- **Accept interfaces, return structs.** Use case в конструкторе принимает интерфейсы (ports), возвращает свой struct.
- **Define interfaces at point of use.** Порты живут в `app/`, рядом с use case, который их потребляет. Не в `adapter/`.
- **Value Objects валидируются в конструкторе.** Если в коде есть `domain.MAC`, она гарантированно валидна.
- **Типизированные доменные ошибки.** `var ErrXxx = errors.New(...)`, проверяются через `errors.Is`. Никаких строковых сравнений.
- **`context.Context` первым параметром** во всех use cases, портах и adapter-методах.
- **Composition только в `cmd/`.** Никакой struct не создаёт свои зависимости сам — всё прокидывается извне.

## Стек

- Go 1.23+
- [`gopkg.in/telebot.v3`](https://github.com/tucnak/telebot) — Telegram bot framework
- [`caarlos0/env/v11`](https://github.com/caarlos0/env) — env-конфиг
- `log/slog` (stdlib) — structured logging
- `testing` (stdlib) — юнит-тесты

## Сборка

```bash
make test              # все тесты
make build             # бинарь для текущей платформы
make build-router      # cross-compile под AX3200 (ARM64 + strip)
make lint              # go vet + опционально golangci-lint
```

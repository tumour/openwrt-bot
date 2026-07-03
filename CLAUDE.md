# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Что это

Telegram-бот для управления домашними роутерами на OpenWrt (бан устройств, per-device VPN-обход, /status, /list, /speedtest). Эталонная демонстрация Hexagonal Architecture на Go — архитектурная чистота здесь важнее скорости добавления фич.

Весь репозиторий на русском: комментарии, коммиты, документация. Коммиты — в стиле conventional commits (`feat:`, `fix:`, `ci:`, `docs:`, `test:`) с русским описанием. **Никаких Co-Authored-By и других AI-трейлеров в коммитах.**

## Команды

```bash
make test          # go test -race -count=1 ./...
make lint          # go vet + gofmt + golangci-lint (если установлен)
make build         # бинарь под текущую платформу → bin/bot
make build-router  # cross-compile ARM64 + strip → bin/bot-arm64 (роутеры aarch64)
make run           # build + запуск локально (нужен env с BOT_TOKEN и т.д.)
```

Один тест: `go test -race -run 'TestBan_Execute' ./internal/app/device/`

Деплой: `make deploy TARGET=<имя>` — таргет это файл `deploy/env/<имя>.env` (gitignored, содержит секреты; ssh-хост в шапке `# DEPLOY_SSH_HOST=...`). Скрипт `deploy/deploy.sh` роутеро-агностичен и идемпотентен: сверяет артефакты через `cmp`, перезапускает бота только при реальных изменениях. Деплой трогает живой роутер — не запускать без явной просьбы пользователя.

## Архитектура

Hexagonal / Ports & Adapters. Подробности и пошаговый рецепт «как добавить use case» — в README.md; история решений и их мотивировка — в ROADMAP.md (новые итерации документируются там же).

**Dependency rule проверяется машинно** тестом `internal/archtest/arch_test.go`:
- `internal/domain/` — только stdlib. Value objects (`MAC` с валидацией в конструкторе), entities, типизированные ошибки (`var ErrXxx`, проверка через `errors.Is`).
- `internal/app/<фича>/` — только domain + stdlib. Вертикальные слайсы по фичам (`device/`, `status/`, `network/`), у каждой свой `ports.go` — интерфейсы объявляются рядом с потребителем, не в adapter.
- `internal/adapter/primary/telegram/` — driving: middleware (auth-whitelist, log, base-context) → handler → use case → presenter.
- `internal/adapter/secondary/{nftables,ubus,dhcp,thermal,librespeed,system}/` — driven: реализации портов через exec/чтение файлов.
- `internal/adapter/secondary/jsondb/` — generic-движок «JSON-файл как коллекция» (конверт `{v, items}`, атомарная запись); `accessjson/` — фичевый адаптер хранилища доступа поверх него (оба порта sub-store'ами `Users()`/`Roles()` под одним локом). Новая сущность = новый фичевый адаптер, в движок и чужие адаптеры не лезем. Смена движка = пакет `accesssqlite` рядом + прогон контрактного сьюта `app/access/accesstest`.
- `internal/platform/` — config (caarlos0/env), logger (slog), graceful shutdown.
- `cmd/bot/main.go` — единственный composition root (паттерн `main → run() error`). Никто больше не создаёт свои зависимости сам.

Граница domain ↔ app: в domain — инварианты, верные вне приложения («broadcast-MAC нельзя банить»); в app — orchestration портов и application rules («повторный бан = no-op»).

## Сквозные соглашения (легко нарушить, читая один файл)

- **Все внешние команды — через `system.Runner`** (`ExecRunner`: типизированная `ExecError` с полем Stderr, `LC_ALL=C`), чтение файлов — через `system.FileReader`. Не звать `exec.Command` напрямую из адаптеров.
- Adapters типизируют доменные ошибки на границе: nftables распознаёт `ErrAlreadyInSet`/`ErrNotInSet` строго по `ExecError.Stderr`.
- **Только HTML-режим Telegram** (`tele.ModeHTML`) и `html.EscapeString` для внешних строк. Markdown в боте запрещён — необэкранированный `_` в имени speedtest-сервера уже ронял Edit (итерация 8 в ROADMAP).
- Регистрация команды — одна строка в `commands()` в `router.go`: единый список питает маршруты, Telegram-меню (SetCommands), reply-клавиатуру И требуемое право. **Каждая команда закрыта Permission** (кроме /start); «нет пермишена = нет кнопки»: меню, клавиатура и inline-кнопки строятся от Grant актора (кладёт auth-middleware). Каталог прав — код (`domain/permission.go`), роли — данные.
- Доступ: env `ADMIN_USER_ID` (ровно один ID) — только сид при старте; пользователи/роли живут в `DATABASE_DIR` и управляются через бота (approve-flow + /users). Use cases, мутирующие доступ, сами проверяют актора (`requireManager`) — guard в роутере лишь UX-слой.
- `context.Context` первым параметром во всех use cases, портах и adapter-методах. Таймауты в handlers строятся от base-context из middleware (иначе ломается graceful shutdown).
- Тесты — только stdlib `testing`, моки пишутся руками рядом с тестом. Тесты слоёв подчиняются тому же dependency rule (archtest обходит и `_test.go`).
- Зависимости: только стабильные релизы, никаких beta/rc без явного согласия пользователя.

## Runtime-контекст (то, что не видно из кода)

Бот крутится на роутерах procd-сервисом: бинарь `/usr/bin/openwrt-bot`, тумблер `/usr/bin/bot` (`bot on/off/restart/status/log`), init.d `deploy/openwrt-bot` bootstrap'ит nft-сеты; конфиг и runtime-состояние — в каталоге `/etc/openwrt-bot/` (env и т.п.), который вписан в `/etc/sysupgrade.conf` и переживает перепрошивку. Два сета в таблице `inet fw4`: `banned_macs` (drop) и `vpn_direct_macs` (mark 0xff — анти-петля xray, трафик мимо TPROXY напрямую через ISP). Правила создаёт init.d, бот управляет **только элементами сетов**. В Telegram бот ходит через socks-прокси (`HTTPS_PROXY`), т.к. api.telegram.org из РФ заблокирован. Пример конфига — `deploy/openwrt-bot.env.example`.

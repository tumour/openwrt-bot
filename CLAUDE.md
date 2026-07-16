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

Деплой: `make deploy TARGET=<имя>` — таргет это файл `deploy/env/<имя>.env` (gitignored, содержит секреты; ssh-хост в шапке `# DEPLOY_SSH_HOST=...`). Скрипт `deploy/deploy.sh` роутеро-агностичен и идемпотентен: сверяет артефакты через `cmp` (хелпер `sync_file`), перезапускает бота только при реальных изменениях; файлы LuCI-панели триггерят не рестарт бота, а `rpcd reload` + сброс `/tmp/luci-indexcache.*.json` + smoke-check `ubus call luci.openwrt-bot status`. Деплой трогает живой роутер — не запускать без явной просьбы пользователя.

## Архитектура

Hexagonal / Ports & Adapters. Подробности и пошаговый рецепт «как добавить use case» — в README.md; история решений и их мотивировка — в ROADMAP.md (новые итерации документируются там же).

**Dependency rule проверяется машинно** тестом `internal/archtest/arch_test.go`:
- `internal/domain/` — только stdlib. Value objects (`MAC` с валидацией в конструкторе), entities, типизированные ошибки (`var ErrXxx`, проверка через `errors.Is`).
- `internal/app/<фича>/` — только domain + stdlib. Вертикальные слайсы по фичам (`device/`, `status/`, `network/`, `timer/`), у каждой свой `ports.go` — интерфейсы объявляются рядом с потребителем, не в adapter.
- `internal/adapter/primary/telegram/` — driving: middleware (auth-whitelist, log, base-context) → handler → use case → presenter.
- `internal/adapter/primary/httpapi/` — driving №2: локальный HTTP API для LuCI-панели поверх тех же use cases. Primary-адаптеры друг о друге не знают — единственная связь (`TelegramUp: bot.Connected`) живёт в composition root.
- `luci-app-openwrt-bot/` — LuCI-панель (раскладка LuCI-feed: `htdocs/` + `root/`): rpcd ucode-плагин `luci.openwrt-bot` (тумблеры через exec + прокси в HTTP API через `uclient-fetch --no-proxy`; НЕ ucode-uclient — вложенный uloop в процессе rpcd опасен), ACL, меню, JS-вьюха. Контракт — `luci-app-openwrt-bot/API.md`. Go-код панель не знает.
- `internal/adapter/secondary/{nftables,ubus,dhcp,thermal,librespeed,system,schedule}/` — driven: реализации портов через exec/чтение файлов; `schedule` — обёртка таймерного движка под порт `timer.SchedulerPort` (трансляция generic↔domain типов и ошибок).
- `internal/platform/` — config (caarlos0/env), logger (slog), graceful shutdown, `scheduler` (обобщённый таймерный движок `Scheduler[J]` — domain-agnostic, как rungroup).
- `cmd/bot/main.go` — единственный composition root (паттерн `main → run() error`). Никто больше не создаёт свои зависимости сам.

Граница domain ↔ app: в domain — инварианты, верные вне приложения («broadcast-MAC нельзя банить»); в app — orchestration портов и application rules («повторный бан = no-op»).

## Сквозные соглашения (легко нарушить, читая один файл)

- **Все внешние команды — через `system.Runner`** (`ExecRunner`: типизированная `ExecError` с полем Stderr, `LC_ALL=C`), чтение файлов — через `system.FileReader`. Не звать `exec.Command` напрямую из адаптеров.
- Adapters типизируют доменные ошибки на границе: nftables распознаёт `ErrAlreadyInSet`/`ErrNotInSet` строго по `ExecError.Stderr`.
- **Только HTML-режим Telegram** (`tele.ModeHTML`) и `html.EscapeString` для внешних строк. Markdown в боте запрещён — необэкранированный `_` в имени speedtest-сервера уже ронял Edit (итерация 8 в ROADMAP).
- Регистрация команды — одна строка в `commands()` в `router.go`: единый список питает и маршруты, и Telegram-меню (SetCommands).
- `context.Context` первым параметром во всех use cases, портах и adapter-методах. Таймауты в handlers строятся от base-context из middleware (иначе ломается graceful shutdown).
- Тесты — только stdlib `testing`, моки пишутся руками рядом с тестом. Тесты слоёв подчиняются тому же dependency rule (archtest обходит и `_test.go`).
- Зависимости: только стабильные релизы, никаких beta/rc без явного согласия пользователя.

## Runtime-контекст (то, что не видно из кода)

Бот крутится на роутерах procd-сервисом: бинарь `/usr/bin/openwrt-bot`, тумблер `/usr/bin/bot` (`bot on/off/restart/status/log`), init.d `deploy/openwrt-bot` bootstrap'ит nft-сеты; конфиг и runtime-состояние — в каталоге `/etc/openwrt-bot/` (env и т.п.), который вписан в `/etc/sysupgrade.conf` и переживает перепрошивку. Баны и vpn-обход — в собственной таблице `inet openwrt_bot`: сеты `banned_macs` (drop) и `vpn_direct_macs` (mark 0xff — анти-петля xray, трафик мимо TPROXY напрямую через ISP), оба правила в base-цепочке `ban_prerouting` (hook prerouting, priority mangle-10 = -160 — раньше TPROXY xray). Лимиты скорости (/limit) — в таблице `netdev openwrt_bot`: map'ы `rate_ul`/`rate_dl` (`ether_addr : limit`) и policing-цепочки `lan_ingress`/`lan_egress` на br-lan (family netdev обязателен — только на ingress/egress доступен MAC). Обе таблицы — собственность бота, `fw4 reload` их не трогает (правила в `inet fw4` он вычищал flush'ем — итерация 14 в ROADMAP). Правила/сеты/map'ы создаёт init.d, бот управляет **только элементами сетов, limit-объектами `lim_{ul,dl}_<12hex>` и элементами map'ов**. В Telegram бот ходит через socks-прокси (`HTTPS_PROXY`), т.к. api.telegram.org из РФ заблокирован. Бот стартует без сети (Offline-конструктор) и дозванивается до Telegram сам с backoff — лежащий VPN или битый токен процесс не убивают, диагноз смотреть в `bot log`. HTTP API для LuCI-панели включается `HTTP_ADDR` (только `127.0.0.1:8787` — auth нет; пусто = выключен). Пример конфига — `deploy/openwrt-bot.env.example`.

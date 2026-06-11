# Roadmap

Откуда идём, куда движемся. Архитектура и принципы — в [README.md](README.md).

## ✓ Сделано

### Итерация 1 — ядро архитектуры
- `domain/` — value object `MAC` с валидацией в конструкторе, entity `Device`, типизированные доменные ошибки.
- `app/` — структура **feature-based** (`app/device/`, `app/status/`), порты живут рядом с use cases.
- Use cases: `GetStatus`, `Ban`, `Unban`, `List` (последний заработал в итерации 4).
- Порты объявлены: `NftPort`, `DhcpPort`, `SystemPort`.

### Итерация 2 — `/status` end-to-end
- `platform/` — config через `caarlos0/env`, logger через `slog`, shutdown через `signal.NotifyContext`.
- `adapter/secondary/system/Runner` + `ExecRunner` — единая точка вызова внешних команд.
- `adapter/secondary/ubus/` — реализация `SystemPort` через `ubus call system info` (uptime/load/memory).
- `adapter/primary/telegram/` — bot lifecycle, middleware (`Auth` whitelist + `Log`), `handler/status`, `presenter/status`.
- `cmd/bot/main.go` — composition root (Mat Ryer style: `main → run`).

### Итерация 3 — `/ban`, `/unban`
- `adapter/secondary/nftables/` — управление сетом `inet fw4 banned_macs` через `nft add/delete/list element`.
- Распознавание `ErrAlreadyBanned` / `ErrNotBanned` по stderr → правильная типизация на границе adapter.
- `handler/ban`, `handler/unban`, `presenter/device` (Banned/Unbanned форматтеры).
- Application rule: повторный бан = no-op (тест `TestBan_Execute_AlreadyBanned_IsNoOp`).

### Итерация 4 — `/list`
- `adapter/secondary/system/FileReader` — **второй** порт на границе OS (ISP: «несколько мелких портов лучше одного раздутого»).
- `adapter/secondary/dhcp/` — парсинг `/tmp/dhcp.leases`. Чистая функция `parseLeases` отделена от I/O (Functional Core, Imperative Shell).
- Use case `List` — orchestration двух портов (`DhcpPort` + `NftPort`).
- `handler/list`, `presenter/list` — моноширинная таблица в Markdown.

### Итерация 5 — температура
- Решение по вопросу из роадмапа: **отдельный `ThermalPort`**, не раздувание `SystemPort`. Источник другой (sysfs-файл, а не ubus), а `GetStatus` оркестрирует оба порта — ровно как `List` оркестрирует `DhcpPort` + `NftPort`.
- `adapter/secondary/thermal/` — reuse `FileReader` для чтения `/sys/class/thermal/thermal_zone0/temp`. Чистая `parseMilliCelsius` (millidegree → °C) отделена от I/O.
- Путь термозоны конфигурируем (`THERMAL_ZONE_PATH`, дефолт `thermal_zone0`) — у разного железа датчик CPU в разных зонах.
- `GetStatus` подмешивает температуру **best-effort**: ошибка датчика (нет зоны / битый файл) НЕ роняет `/status`, `TempCelsius` остаётся 0, presenter строку скрывает.
- presenter `/status` уже умел `*Temp:* X°C` при `>0` — правок не потребовал.

### Итерация 6 — `/speedtest`
- Новая фича `app/network/` — порт `SpeedTestPort` + DTO `SpeedResult` (download/upload Mbps, ping/jitter ms, server). Use case `RunSpeedTest`.
- Инструмент: **librespeed-cli** (apk-пакет на OpenWrt, выбран против iperf3 — нулевая инфра, нет открытых портов; против speedtest-netperf — чистый `--json` вместо парсинга текста). Адаптер `adapter/secondary/librespeed` через общий `system.Runner`, парсит JSON-массив.
- Сервер пиннуем через `SPEEDTEST_SERVER_ID` (пусто = авто; авто часто берёт далёкий сервер → заниженные цифры).
- `handler/speedtest` — отдельный таймаут 90с (замер ~30-60с) + interim-сообщение «⏳ замеряю…» с последующим `Edit` результатом. `presenter/speedtest` — Mbps/ping/jitter/сервер.
- **Нюанс маршрута**: замер с роутера идёт напрямую через ISP (TPROXY ловит только форвард LAN, не output роутера) → меряем домашний канал, не VPN.

### Меню команд (SetCommands)
- Нативное меню Telegram (кнопка ≡ + автодополнение по «/») через `bot.SetCommands`.
- **Единый источник** `commands()` в `router.go`: один список `[]command{name, desc, handle, inMenu}` питает И маршруты (`registerRoutes`), И меню (`menuCommands`) — добавление команды остаётся одной строкой, списки не разъезжаются.
- `/start` помечен `inMenu:false` (алиас `/status`, в меню не нужен). Установка меню в `NewBot` — best-effort (ошибка API логируется, старт не валит). Тест на ограничения Telegram (имя `[a-z0-9_]`≤32, описание 3-256 символов).

### Деплой на роутер
- Роутер живой: OpenWrt 25.12.4 (aarch64), бот крутится procd-сервисом, управление тумблером `bot on/off/restart/status/log`.
- `make deploy` — одна команда: `build-router` → scp на роутер → `bot off` → подмена `/usr/bin/home-monitor` → `bot on`. init.d и тумблер заливаются только если изменились (`cmp`).
- Доступ: ssh-алиас `router` в `~/.ssh/config` (100.64.0.2 через jump-VPS), авторизация по ключу.
- Нюанс: dropbear на OpenWrt без sftp-server → scp с флагом `-O` (легаси-протокол).
- env на роутере (`/etc/openwrt-bot.env`) переименован под `ALLOWED_USER_IDS` (2026-06-11).

## Следующее

### Дальше (когда понадобится)
- `/reboot` — управляемый рестарт с подтверждением кнопкой.
- `/wifi` — включить/выключить, поменять пароль, гостевая сеть.
- Per-device traffic counters через `nft` `counter` объекты.
- Расписания (бан на N минут, ночной off для детских устройств) — нужен `ClockPort` + cron-like тикер.
- Prometheus endpoint, если захочется Grafana.

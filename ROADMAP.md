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

## Следующее

### Итерация 5 — температура
- Reuse `FileReader` для чтения `/sys/class/thermal/thermal_zone0/temp`.
- Расширить `status.SystemPort.Snapshot()` температурой (или отдельный `ThermalPort`?).
- Обновить presenter `/status`: строка `*Temp:* X°C`.
- Тесты.

### Итерация 6 — `/speedtest`
- Новая фича: `app/network/`.
- Порт `SpeedTestPort` + adapter через `iperf3 -c <server>` или `librespeed-cli`.
- `handler/speedtest`, presenter с форматированием Mbps.

### Развёртывание на AX3200
- Прошить роутер на OpenWrt 23.05+ (отдельная задача — гайд под AX3200 через USB-TTL/exploit).
- `make build-router` — cross-compile под ARM64 с `-s -w`.
- `/etc/init.d/openwrt-bot` — procd-сервис для автозапуска.
- Bootstrap nftables: создать set `banned_macs` + правило `drop ether saddr @banned_macs` в `inet fw4 forward`.
- `.env` на роутере: `BOT_TOKEN`, `ALLOWED_CHAT_IDS`, опционально `LOG_LEVEL`.

### Дальше (когда понадобится)
- `/reboot` — управляемый рестарт с подтверждением кнопкой.
- `/wifi` — включить/выключить, поменять пароль, гостевая сеть.
- Per-device traffic counters через `nft` `counter` объекты.
- Расписания (бан на N минут, ночной off для детских устройств) — нужен `ClockPort` + cron-like тикер.
- Prometheus endpoint, если захочется Grafana.

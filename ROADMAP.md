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

### Деплой на роутер (мульти-роутер, 2026-06-19)
- Роутеры живые: OpenWrt aarch64, бот крутится procd-сервисом, управление тумблером `bot on/off/restart/status/log`.
- **Конфиг каждого роутера — отдельный `deploy/env/<target>.env`** (gitignored, секреты, chmod 600); в шапке `# DEPLOY_SSH_HOST=<ssh>` — куда коннектиться (init.d строки с `#` игнорит). Добавить роутер = создать один env-файл.
- `make deploy TARGET=<target>` (или `./deploy/deploy.sh <target>`): `build-router` → scp во временные файлы → подмена только изменившихся артефактов. Скрипт **роутеро-агностичен** (имён роутеров в коде нет, ssh-host берётся из env), **идемпотентен** (всё через `cmp`, повтор без правок — no-op, работающего бота не трогает) и **универсален** (нет бота → ставит и запускает; есть → обновляет, `bot off` перед подменой бинаря из-за "text file busy").
- env синкается: локальный `deploy/env/<t>.env` — источник истины, `/etc/openwrt-bot/env` перезаписывается при отличии.
- **Каталог бота на роутере (2026-07-02):** конфиг и runtime-состояние собраны в `/etc/openwrt-bot/` (env; будущий users.json туда же) — по образцу `/etc/xray/`. Deploy идемпотентно вписывает каталог в `/etc/sysupgrade.conf` — переживает перепрошивку целиком. Бинарь и тумблер остаются в `/usr/bin`, init.d — в `/etc/init.d` (конвенция OpenWrt, не разбросанность). Старый `/etc/openwrt-bot.env` после первого деплоя новой схемы удалить руками.
- **Бинарь переименован (2026-07-02):** `/usr/bin/home-monitor` → `/usr/bin/openwrt-bot` — историческое имя расходилось с репо/сервисом/каталогом; заодно syslog-тег стал `openwrt-bot`, и `bot log` грепает родное имя. Оба роутера мигрированы одноразовым блоком в deploy.sh (стоп старого процесса и удаление старого бинаря ДО установки нового — иначе два инстанса на одном токене → Telegram 409); блок выпилен сразу после миграции, в истории репо его нет.
- Доступ: AX3200 — ssh-алиас `router` (100.64.0.2 через jump-VPS, awg-mesh); Flint 2 — `root@192.168.1.1` напрямую (awg-mesh там пока нет). Авторизация по ключу.
- Нюанс: dropbear на OpenWrt без sftp-server → scp с флагом `-O` (легаси-протокол).
- env-ключ доступа — `ALLOWED_USER_IDS` (переименован 2026-06-11).

### Итерация 7 — /vpnoff, /vpnon (vpn-обход per-device)
- Идея: xray-цепочка `ip xray prerouting` пропускает пакеты с меткой 0xff (`meta mark 0xff return` — его же анти-петля). Наша цепочка `ban_prerouting` (priority -160) срабатывает раньше xray (-150) → метим трафик выбранных MAC, и устройство ходит напрямую через ISP.
- Сет `vpn_direct_macs` в `inet fw4` (bootstrap в init.d, как banned_macs) — переживает `vpn off/on`, чистится ребутом. Таблица `ip xray` и репо openwrt-vpn не тронуты.
- Рефакторинг: `NftPort` → generic `MACSetPort` (Add/Remove/List), доменные ошибки `ErrAlreadyInSet`/`ErrNotInSet`; nftables.Client один, экземпляра два (banned + direct). Use cases `DisableVPN`/`EnableVPN` — копия паттерна Ban/Unban с no-op на повторе.
- `/list` помечает обходящие устройства: 🌐 + «без VPN». Бан сильнее обхода (drop стоит в цепочке первым).
- Нюанс: DNS у «прямого» устройства при включённом VPN остаётся через xray DoH (dnsmasq общий на LAN) — трафик прямой, резолв честный. IPv6 при VPN on глушится для всего LAN, обходное устройство живёт на v4.

### Итерация 8 — надёжность (2026-06-11)
- **Баг:** имя speedtest-сервера вклеивалось в Markdown без экранирования — «_»/«*» в имени → Telegram 400 → Edit падал, юзер навсегда оставался с «⏳ замеряю…». Заодно все сообщения переведены на единый HTML-режим (`tele.ModeHTML`): у HTML типизированное экранирование (`html.EscapeString`) для внешних строк. Markdown в боте больше не используется.
- **Баг:** graceful shutdown не доходил до handlers — таймауты строились от `context.Background()`, telebot `Stop()` in-flight горутины не ждёт. Теперь track-middleware кладёт базовый ctx из `Run` в `telebot.Context` (`middleware.BaseContext`), все handler-таймауты строятся от него, `Bot.Run` дожидается in-flight (WaitGroup, потолок 5с). Это же было причиной долгого `bot off` в deploy.
- `system.Runner` → типизированная `ExecError{Name, Args, Stderr, Err}`: nftables типизирует `ErrAlreadyInSet`/`ErrNotInSet` строго по полю Stderr (раньше — подстрока по всему Error() вместе с аргументами команды). `LC_ALL=C` в exec — независимость от локали. `Unwrap` сохраняет `errors.Is(err, exec.ErrNotFound)` для librespeed.
- `editKeepalive` узнаёт «message is not modified» типизированно (`tele.ErrSameMessageContent`/`ErrMessageNotModified`), подстрока — страховка.
- Dependency rule проверяется машинно: `internal/archtest` (domain → только stdlib; app → только domain). GitHub Actions: `make lint` + `make test` на каждый push/PR.
- **Решение:** контракт `MACSetPort` с типизированными ошибками сознательно НЕ заменён на идемпотентный — «повторный бан = no-op» остаётся демонстрацией application rule в app-слое (см. README). Пересмотреть, если у Ban появится второе действие (deauth и т.п.).

### Итерация 9 — /limit, /unlimit (per-device лимит скорости) (2026-07-04)
- Идея: policing средствами nftables — пакеты сверх N КБ/с дропаются, TCP сам сбавляет темп. Без tc/HTB: шейпинг точнее (очереди вместо дропа), но требует qdisc на br-lan + IFB + классы + фильтры и целый новый класс адаптеров — не оправдано для домашнего «притормозить устройство». Ожидание точности: TCP держится на ~80–95% планки, «пилой»; UDP-стримы режутся жёстко.
- **Схема:** отдельная таблица `netdev openwrt_bot` — family netdev обязателен, т.к. download по MAC ловится только на **egress br-lan** (только там доступен `ether daddr`; в forward/postrouting L3-таблиц ethernet-заголовка ещё нет), upload — на ingress (`ether saddr`). Бонусы схемы: bridged LAN↔LAN трафик через эти хуки не проходит (внутрисетевые копирования не страдают), tproxy-нутый xray-трафик виден (ingress раньше prerouting, egress видит local output), netdev-таблицу не трогает `fw4 reload` (в отличие от сетов в fw4). Map'ы `rate_ul`/`rate_dl` (`ether_addr : limit`) + именованные limit-объекты `lim_{ul,dl}_<12hex>`; одно значение из /limit режет каждое направление независимо. Требования (проверены на обоих роутерах, OpenWrt 25.12): nft с `destroy` (≥1.1) и **выключенный flow offloading** — offloaded-потоки идут мимо netdev-хуков.
- **Контраст контрактов портов** (сознательный): `RateLimitPort` идемпотентный — Set = создать-или-обновить, Remove отсутствующего = no-op уже в адаптере, — потому что идемпотентность даёт сам механизм: вся мутация — одна атомарная nft-транзакция (semicolon-joined script одним argv) из `destroy`-команд (delete-if-exists) + `add`, которая заодно самовосстанавливает частично снесённое руками состояние. Типизированный `MACSetPort` (ит. 8) остаётся демонстрацией application rule в app-слое; здесь же типизировать нечего — no-op некому интерпретировать.
- Burst у limit-объектов: полсекунды трафика, clamp [16..2048] КБ (`burstKB` в адаптере). Burst≈0 у byte-limit дропает пакетные пачки TCP и роняет скорость сильно ниже планки. Подобрано умозрительно — уточнить по замерам.
- Лимиты в `/list` — best-effort (прецедент: температура в /status): старый init.d без netdev-таблицы не роняет список, ошибка всплывёт на /limit. Карточка: ряд пресетов 256/512/1024/2048 КБ/с (payload `mac|rate`, ≤32 байт из 64 доступных в callback data; активный помечен ✓) + «Снять лимит» при активном лимите. Иконки: 🚫 > ⏱ > 🌐.
- Нюанс парсинга `nft list table`: nft укрупняет единицы при печати (лимит 1024 КБ/с печатается «1 mbytes/second») — regex покрывает bytes|kbytes|mbytes|gbytes с конвертацией в КБ; MAC восстанавливается из имени limit-объекта. Fixture в тестах снят с реального вывода (netns-песочница).

## Следующее

### Дальше (когда понадобится)
- `/reboot` — управляемый рестарт с подтверждением кнопкой.
- `/wifi` — включить/выключить, поменять пароль, гостевая сеть.
- Per-device traffic counters через `nft` `counter` объекты.
- Расписания (бан на N минут, ночной off для детских устройств) — нужен `ClockPort` + cron-like тикер.
- Prometheus endpoint, если захочется Grafana.

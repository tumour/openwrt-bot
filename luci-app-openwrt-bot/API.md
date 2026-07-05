# Контракты панели

Панель — тонкий клиент: браузер → ubus (`luci.openwrt-bot`, rpcd ucode-плагин) →
HTTP API бота (`127.0.0.1:8787`). Этот файл — единственный документ контракта
между слоями; истина по HTTP-стороне — `internal/adapter/primary/httpapi/`.

## 1. HTTP API бота

Включается env `HTTP_ADDR=127.0.0.1:8787` (пусто = выключен). Аутентификации
нет — слушать только loopback. Успех мутаций — всегда `200 {"ok":true}`;
ошибки — `{"error":"текст"}`; 404/405 — plain-text от stdlib ServeMux
(машинный контракт — статус-код). Пустые списки сериализуются `[]`, не `null`.

| Метод | Путь | Тело запроса | Успех | Ошибки |
|---|---|---|---|---|
| GET | `/api/v1/health` | — | `{"status":"ok","telegram":"connected"\|"connecting"}` | — |
| GET | `/api/v1/devices` | — | `{"devices":[deviceJSON…]}` | 500 |
| POST | `/api/v1/devices/{mac}/ban` | — | `{"ok":true}` | 400 (MAC), 500 |
| POST | `/api/v1/devices/{mac}/unban` | — | `{"ok":true}` | 400, 500 |
| POST | `/api/v1/devices/{mac}/vpnoff` | — | `{"ok":true}` | 400, 500 |
| POST | `/api/v1/devices/{mac}/vpnon` | — | `{"ok":true}` | 400, 500 |
| POST | `/api/v1/devices/{mac}/unlimit` | — | `{"ok":true}` | 400, 500 |
| POST | `/api/v1/devices/{mac}/limit` | `{"kbytes_per_sec":1..1000000}` | `{"ok":true}` | 400 (MAC/JSON/поле/хвост/диапазон), 413 (>1КиБ), 500 |

`deviceJSON`: `mac` (нормализованный), `hostname`, `ip` (`""` = неизвестен),
`banned`, `direct` (мимо VPN), `limit_kbytes_per_sec` (0 = нет лимита).
Единица лимита — **КБайт/с** (язык nftables `kbytes/second`), сознательно
не «kbps»: то читается как килобиты/с и порождает ×8-баги.
MAC в пути принимается в любом регистре и с `-`-разделителями — бот нормализует.

## 2. ubus-объект `luci.openwrt-bot`

Плагин: `root/usr/share/rpcd/ucode/openwrt-bot`. Два слоя: control plane
(exec тумблеров — работает и при мёртвом боте), device plane (прокси в HTTP
API через `uclient-fetch --no-proxy`, таймаут 5 с). Активация после установки:
`/etc/init.d/rpcd reload && rm -f /tmp/luci-indexcache.*.json`.

| Метод | ACL | Аргументы | Ответ |
|---|---|---|---|
| `status` | read | — | `{bot:{running,pid},vpn:{installed,up},api:{up},telegram}` |
| `devices` | read | — | `{"devices":[deviceJSON…]}` или `{"error":…}` |
| `bot` | write | `action` ∈ on/off/restart | `{ok:bool, output:string}` |
| `vpn` | write | `action` ∈ on/off | `{ok:bool, output:string}` или `{"error":"vpn_not_installed"}` |
| `device_action` | write | `mac`, `action` ∈ ban/unban/vpnoff/vpnon/unlimit | `{"ok":true}` или `{"error":…}` |
| `set_limit` | write | `mac`, `kbytes_per_sec` (int 1..1000000) | `{"ok":true}` или `{"error":…}` |

Предикаты `status`: бот — `pgrep -f /usr/bin/openwrt-bot` (паритет тумблера
`bot`); VPN установлен — `access('/usr/bin/vpn')` (feature detection, репо
openwrt-vpn); VPN поднят — socks `127.0.0.1:10808` слушает; `telegram` —
прокси `/health`, `null` если API недоступен.

Словарь ошибок плагина:

| Код | Значение |
|---|---|
| `bot_down` | HTTP API недоступен (процесс мёртв, HTTP_ADDR пуст, таймаут) |
| `http_<N>` | API ответил статусом N ≥ 400 (тело недоступно — uclient-fetch его не отдаёт) |
| `bad_json` | API ответил не-JSON |
| `bad_action` | действие вне whitelist |
| `bad_mac` | MAC не прошёл регэксп (валидация до подстановки в URL) |
| `bad_rate` | лимит вне 1..1000000 или не int |
| `vpn_not_installed` | нет `/usr/bin/vpn` |

Санитация: в shell и URL попадают только элементы whitelist и MAC после
регэкспа — пользовательский ввод не подставляется никуда.

Проверка руками: `ubus call luci.openwrt-bot status`,
`ubus call luci.openwrt-bot device_action '{"mac":"aa:bb:cc:11:22:33","action":"ban"}'`.

#!/bin/sh
# Деплой openwrt-bot на роутер. Скрипт РОУТЕРО-АГНОСТИЧЕН: ни одного имени
# роутера в коде — вся специфика живёт в deploy/env/<target>.env (gitignored).
#
# Использование: ./deploy/deploy.sh <target>      (или make deploy TARGET=<target>)
#   <target> = имя env-файла в deploy/env/<target>.env.
#   КУДА коннектиться берётся из шапки этого файла:  # DEPLOY_SSH_HOST=<ssh>
#   (ssh-алиас или root@ip). init.d строки с # игнорирует — метаданные на роутере
#   безвредны. Добавить роутер = создать deploy/env/<name>.env с этой шапкой;
#   сам скрипт трогать не нужно.
#
# ИДЕМПОТЕНТЕН: каждый артефакт сверяется через cmp и заливается только если
# реально изменился. Бота останавливаем/перезапускаем ТОЛЬКО при изменениях.
# Повторный запуск без правок — no-op, работающего бота не трогает.
#
# УНИВЕРСАЛЕН: бота ещё нет → ставит и запускает; уже есть → обновляет и
# перезапускает (bot off перед подменой бинаря — иначе "text file busy").
# Баны (nft-set) переживают рестарт: init.d при stop сет не трогает.
#
# Заодно деплоит LuCI-панель (luci-app-openwrt-bot/): её файлы триггерят
# НЕ рестарт бота, а `rpcd reload` + сброс кэша LuCI — и только при изменениях.
set -eu
cd "$(dirname "$0")/.."

ENV_DIR=deploy/env
ssh_host_of() { sed -n 's/^#[[:space:]]*DEPLOY_SSH_HOST=//p' "$1" | head -1; }

usage() {
	echo "usage: $0 <target>" >&2
	echo "доступные таргеты (deploy/env/*.env):" >&2
	found=0
	for f in "$ENV_DIR"/*.env; do
		[ -e "$f" ] || break
		found=1
		printf '  %-12s → %s\n' "$(basename "$f" .env)" "$(ssh_host_of "$f")" >&2
	done
	[ "$found" = 1 ] || echo "  (пусто — создай $ENV_DIR/<name>.env с шапкой '# DEPLOY_SSH_HOST=<ssh>')" >&2
	exit 1
}

TARGET=${1:-}
[ -n "$TARGET" ] || usage
ENV_FILE="$ENV_DIR/$TARGET.env"
[ -f "$ENV_FILE" ] || { echo "нет $ENV_FILE" >&2; usage; }
HOST=$(ssh_host_of "$ENV_FILE")
[ -n "$HOST" ] || { echo "в $ENV_FILE нет строки '# DEPLOY_SSH_HOST=<ssh>'" >&2; exit 1; }

make build-router

LUCI=luci-app-openwrt-bot

echo "==> $TARGET ($HOST): заливаю во временные файлы"
# -O — легаси scp: dropbear на OpenWrt без sftp-server.
scp -O -q bin/bot-arm64      "$HOST":/tmp/openwrt-bot.bin.new
scp -O -q "$ENV_FILE"        "$HOST":/tmp/openwrt-bot.env.new
scp -O -q deploy/openwrt-bot "$HOST":/tmp/openwrt-bot.new
scp -O -q deploy/bot         "$HOST":/tmp/bot.new
scp -O -q "$LUCI"/root/usr/share/rpcd/ucode/openwrt-bot                "$HOST":/tmp/luci-rpcd.new
scp -O -q "$LUCI"/root/usr/share/rpcd/acl.d/luci-app-openwrt-bot.json  "$HOST":/tmp/luci-acl.new
scp -O -q "$LUCI"/root/usr/share/luci/menu.d/luci-app-openwrt-bot.json "$HOST":/tmp/luci-menu.new
scp -O -q "$LUCI"/htdocs/luci-static/resources/view/openwrt-bot.js     "$HOST":/tmp/luci-view.new

echo "==> применяю только изменения"
# changed=1 — что-то поменялось вообще; restart=1 — нужен рестарт без подмены
# бинаря (env/init.d сменились, а бинарь нет). Бинарь обрабатывается отдельно,
# т.к. его подмена требует предварительного `bot off` (text file busy).
ssh "$HOST" '
	set -e
	changed=0; restart=0; luci=0

	# sync_file <tmp> <dst> <mode> — cmp→mkdir→mv→chmod. Return 0 = файл
	# изменился (флаги ставит вызывающий: `if sync_file …; then …=1; fi` —
	# безопасно под set -e), 1 = не отличался (tmp уберёт общий rm в конце).
	sync_file() {
		cmp -s "$1" "$2" && return 1
		mkdir -p "${2%/*}"
		mv "$1" "$2"; chmod "$3" "$2"
		echo "    ~ $2"
	}

	# Тумблер — пользовательский CLI, рестарта бота НЕ требует.
	if sync_file /tmp/bot.new /usr/bin/bot 755; then changed=1; fi

	# init.d и env — свободные файлы, подменяем на лету, но требуют рестарта,
	# чтобы procd подхватил новый init.d / бот перечитал env.
	if sync_file /tmp/openwrt-bot.new /etc/init.d/openwrt-bot 755; then changed=1; restart=1; fi
	# Конфиг и state бота — в каталоге /etc/openwrt-bot/ (env, будущий users.json).
	if sync_file /tmp/openwrt-bot.env.new /etc/openwrt-bot/env 600; then changed=1; restart=1; fi
	# Одна строка в бэкап-списке sysupgrade — весь каталог переживает перепрошивку.
	if ! grep -qx "/etc/openwrt-bot/" /etc/sysupgrade.conf 2>/dev/null; then
		echo "/etc/openwrt-bot/" >> /etc/sysupgrade.conf
		echo "    + /etc/sysupgrade.conf: /etc/openwrt-bot/"; changed=1
	fi

	# LuCI-панель: файлы безопасно класть в любой момент (rpcd отдаёт старый
	# плагин до reload) — сам триггер после блока бинаря.
	if sync_file /tmp/luci-rpcd.new /usr/share/rpcd/ucode/openwrt-bot 644;                then changed=1; luci=1; fi
	if sync_file /tmp/luci-acl.new /usr/share/rpcd/acl.d/luci-app-openwrt-bot.json 644;   then changed=1; luci=1; fi
	if sync_file /tmp/luci-menu.new /usr/share/luci/menu.d/luci-app-openwrt-bot.json 644; then changed=1; luci=1; fi
	if sync_file /tmp/luci-view.new /www/luci-static/resources/view/openwrt-bot.js 644;   then changed=1; luci=1; fi

	# Бинарь нельзя подменить на работающем (text file busy) → сначала bot off.
	# `bot off` на незапущенном боте — no-op, поэтому работает и для установки с нуля.
	if ! cmp -s /tmp/openwrt-bot.bin.new /usr/bin/openwrt-bot; then
		bot off
		mv /tmp/openwrt-bot.bin.new /usr/bin/openwrt-bot; chmod 755 /usr/bin/openwrt-bot
		echo "    ~ /usr/bin/openwrt-bot"
		bot on          # поднимет бота и подхватит новый env/init.d заодно
		changed=1; restart=0
	elif [ "$restart" = 1 ]; then
		bot restart     # бинарь тот же, но env/init.d сменились
	fi

	# Панель активируется после бинаря: smoke-check status осмыслен при живом
	# боте. reload (SIGHUP) перечитывает плагины и ACL без рестарта демона,
	# но асинхронно — отсюда sleep. Кэш меню LuCI пересобирается сам.
	if [ "$luci" = 1 ]; then
		/etc/init.d/rpcd reload
		rm -f /tmp/luci-indexcache.*.json
		sleep 1
		if ubus call luci.openwrt-bot status 2>/dev/null | grep -q "\"bot\""; then
			echo "    ✓ панель: ubus luci.openwrt-bot отвечает"
		else
			echo "    ✗ панель: luci.openwrt-bot не отвечает — logread | grep rpcd"
		fi
	fi

	rm -f /tmp/openwrt-bot.bin.new /tmp/openwrt-bot.env.new /tmp/openwrt-bot.new /tmp/bot.new \
	      /tmp/luci-rpcd.new /tmp/luci-acl.new /tmp/luci-menu.new /tmp/luci-view.new
	[ "$changed" = 1 ] || echo "    ничего не изменилось — бот не тронут"
'

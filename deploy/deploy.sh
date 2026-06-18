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

echo "==> $TARGET ($HOST): заливаю во временные файлы"
# -O — легаси scp: dropbear на OpenWrt без sftp-server.
scp -O -q bin/bot-arm64      "$HOST":/tmp/home-monitor.new
scp -O -q "$ENV_FILE"        "$HOST":/tmp/openwrt-bot.env.new
scp -O -q deploy/openwrt-bot "$HOST":/tmp/openwrt-bot.new
scp -O -q deploy/bot         "$HOST":/tmp/bot.new

echo "==> применяю только изменения"
# changed=1 — что-то поменялось вообще; restart=1 — нужен рестарт без подмены
# бинаря (env/init.d сменились, а бинарь нет). Бинарь обрабатывается отдельно,
# т.к. его подмена требует предварительного `bot off` (text file busy).
ssh "$HOST" '
	set -e
	changed=0; restart=0

	# Тумблер — пользовательский CLI, рестарта бота НЕ требует.
	if ! cmp -s /tmp/bot.new /usr/bin/bot; then
		mv /tmp/bot.new /usr/bin/bot; chmod 755 /usr/bin/bot
		echo "    ~ /usr/bin/bot"; changed=1
	fi

	# init.d и env — свободные файлы, подменяем на лету, но требуют рестарта,
	# чтобы procd подхватил новый init.d / бот перечитал env.
	if ! cmp -s /tmp/openwrt-bot.new /etc/init.d/openwrt-bot; then
		mv /tmp/openwrt-bot.new /etc/init.d/openwrt-bot; chmod 755 /etc/init.d/openwrt-bot
		echo "    ~ /etc/init.d/openwrt-bot"; changed=1; restart=1
	fi
	if ! cmp -s /tmp/openwrt-bot.env.new /etc/openwrt-bot.env; then
		mv /tmp/openwrt-bot.env.new /etc/openwrt-bot.env; chmod 600 /etc/openwrt-bot.env
		echo "    ~ /etc/openwrt-bot.env"; changed=1; restart=1
	fi

	# Бинарь нельзя подменить на работающем (text file busy) → сначала bot off.
	# `bot off` на незапущенном боте — no-op, поэтому работает и для установки с нуля.
	if ! cmp -s /tmp/home-monitor.new /usr/bin/home-monitor; then
		bot off
		mv /tmp/home-monitor.new /usr/bin/home-monitor; chmod 755 /usr/bin/home-monitor
		echo "    ~ /usr/bin/home-monitor"
		bot on          # поднимет бота и подхватит новый env/init.d заодно
		changed=1; restart=0
	elif [ "$restart" = 1 ]; then
		bot restart     # бинарь тот же, но env/init.d сменились
	fi

	rm -f /tmp/home-monitor.new /tmp/openwrt-bot.env.new /tmp/openwrt-bot.new /tmp/bot.new
	[ "$changed" = 1 ] || echo "    ничего не изменилось — бот не тронут"
'

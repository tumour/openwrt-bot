#!/bin/sh
# Деплой openwrt-bot на роутер одной командой: `make deploy`.
#
# Требует ssh-алиас `router` в ~/.ssh/config (доступ через jump-VPS, ключ).
# Повторяет текущую схему роутера, ничего в ней не меняя:
#   bin/bot-arm64       → /usr/bin/home-monitor   (бинарь бота)
#   deploy/openwrt-bot  → /etc/init.d/openwrt-bot (procd-сервис + bootstrap nft)
#   deploy/bot          → /usr/bin/bot            (тумблер on/off/restart/status/log)
# init.d и тумблер обновляются только если реально изменились (cmp).
#
# Бинарь сначала в /tmp, потом mv после `bot off` — scp поверх работающего
# бинаря падает с "text file busy". Баны (nft-set) рестарт переживают:
# init.d при stop сет не трогает.
set -eu
cd "$(dirname "$0")/.."

HOST=${1:-router}

make build-router

echo "==> заливаю на $HOST"
# -O — легаси scp-протокол: dropbear на OpenWrt без sftp-server,
# а новый openssh-scp по умолчанию ходит по SFTP.
scp -O -q bin/bot-arm64 "$HOST":/tmp/home-monitor.new
scp -O -q deploy/openwrt-bot deploy/bot "$HOST":/tmp/

echo "==> устанавливаю и перезапускаю"
ssh "$HOST" '
	set -e
	cmp -s /tmp/openwrt-bot /etc/init.d/openwrt-bot || {
		mv /tmp/openwrt-bot /etc/init.d/openwrt-bot
		chmod 755 /etc/init.d/openwrt-bot
		echo "    обновлён /etc/init.d/openwrt-bot"
	}
	cmp -s /tmp/bot /usr/bin/bot || {
		mv /tmp/bot /usr/bin/bot
		chmod 755 /usr/bin/bot
		echo "    обновлён /usr/bin/bot"
	}
	rm -f /tmp/openwrt-bot /tmp/bot
	bot off
	mv /tmp/home-monitor.new /usr/bin/home-monitor
	chmod 755 /usr/bin/home-monitor
	bot on
'

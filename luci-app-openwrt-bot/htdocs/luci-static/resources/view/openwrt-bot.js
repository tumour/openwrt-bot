'use strict';
'require view';
'require ui';
'require rpc';
'require poll';
'require dom';

/*
 * Панель openwrt-bot: карточки служб (бот / VPN / Telegram) + таблица
 * устройств LAN. Данные — ubus-объект luci.openwrt-bot (rpcd ucode-плагин,
 * контракт в API.md): control plane работает всегда, device plane — пока
 * жив HTTP API бота.
 *
 * i18n сознательно нет: приложение личное, строки по-русски хардкодом
 * (решение зафиксировано в ROADMAP, итерация 12).
 */

var callStatus = rpc.declare({ object: 'luci.openwrt-bot', method: 'status' });
var callDevices = rpc.declare({ object: 'luci.openwrt-bot', method: 'devices' });
var callBot = rpc.declare({ object: 'luci.openwrt-bot', method: 'bot', params: ['action'] });
var callVpn = rpc.declare({ object: 'luci.openwrt-bot', method: 'vpn', params: ['action'] });
var callDeviceAction = rpc.declare({ object: 'luci.openwrt-bot', method: 'device_action', params: ['mac', 'action'] });
var callSetLimit = rpc.declare({ object: 'luci.openwrt-bot', method: 'set_limit', params: ['mac', 'kbytes_per_sec'] });

var LIMIT_PRESETS = [256, 512, 1024, 2048];

function pill(kind, text) {
	var colors = { on: '#2e9e44', off: '#ca3b3b', wait: '#b07f10', na: '#888' };
	return E('span', { style: 'font-weight:bold; color:' + (colors[kind] || colors.na) }, [ '● ' + text ]);
}

function chip(text) {
	return E('span', {
		style: 'display:inline-block; padding:1px 6px; margin:1px 3px 1px 0; ' +
			'border-radius:3px; background:#8882; white-space:nowrap; font-size:90%'
	}, [ text ]);
}

function card(title, pillNode, bodyLines, buttons) {
	return E('div', { 'class': 'cbi-section', style: 'flex:1 1 250px; margin:0; padding:0.8em 1em' }, [
		E('div', { style: 'display:flex; align-items:center; justify-content:space-between; gap:0.5em' }, [
			E('strong', {}, [ title ]), pillNode
		]),
		E('div', { style: 'color:#888; font-size:90%; min-height:2.4em; margin:0.4em 0' }, bodyLines),
		E('div', { style: 'display:flex; gap:0.4em; flex-wrap:wrap' }, buttons)
	]);
}

return view.extend({
	handleSave: null,
	handleSaveApply: null,
	handleReset: null,

	load: function() {
		return Promise.all([
			L.resolveDefault(callStatus(), {}),
			L.resolveDefault(callDevices(), { error: 'bot_down' })
		]);
	},

	render: function(data) {
		var services = E('div', { id: 'bot-services' }, [ this.renderServices(data[0]) ]);
		var devices = E('div', { id: 'bot-devices' }, [ this.renderDevices(data[0], data[1]) ]);

		poll.add(L.bind(this.refresh, this), 5);

		return E([], [
			E('h2', {}, [ 'OpenWrt Bot' ]),
			E('div', { 'class': 'cbi-map-descr' }, [
				'Управление ботом, VPN и устройствами LAN. Telegram и панель используют одни и те же операции.'
			]),
			E('h3', {}, [ 'Службы' ]),
			services,
			E('h3', {}, [ 'Устройства в LAN' ]),
			devices
		]);
	},

	refresh: function() {
		var self = this;
		return Promise.all([
			L.resolveDefault(callStatus(), {}),
			L.resolveDefault(callDevices(), { error: 'bot_down' })
		]).then(function(data) {
			dom.content(document.getElementById('bot-services'), self.renderServices(data[0]));
			dom.content(document.getElementById('bot-devices'), self.renderDevices(data[0], data[1]));
		});
	},

	// notify — единый исход действий: тумблеры плагина возвращают
	// {ok, output}, прокси-методы — {"ok":true} бота либо {error}.
	notify: function(okMessage) {
		var self = this;
		return function(res) {
			if (!res || res.error || res.ok === false) {
				var reason = res && (res.error || res.output) || 'нет ответа';
				ui.addNotification(null, E('p', {}, [ 'Ошибка: ' + reason ]), 'danger');
			} else if (okMessage) {
				ui.addNotification(null, E('p', {}, [ okMessage ]));
			}
			return self.refresh();
		};
	},

	handleBot: function(action) {
		return callBot(action).then(this.notify(null));
	},

	handleVpn: function(action) {
		return callVpn(action).then(this.notify(null));
	},

	handleDeviceAction: function(mac, action, okMessage) {
		return callDeviceAction(mac, action).then(this.notify(okMessage));
	},

	renderServices: function(st) {
		var readonly = !L.hasViewPermission() || null;
		var bot = st.bot || {};
		var vpn = st.vpn || {};
		var api = st.api || {};

		var botCard = card('🤖 Бот',
			bot.running ? pill('on', 'запущен') : pill('off', 'остановлен'),
			bot.running
				? [ 'pid ' + bot.pid + (api.up ? ' · HTTP API отвечает' : ' · HTTP API выключен') ]
				: [ 'procd-сервис openwrt-bot остановлен' ],
			[
				E('button', {
					'class': 'btn cbi-button cbi-button-positive',
					disabled: readonly || bot.running || null,
					click: ui.createHandlerFn(this, 'handleBot', 'on')
				}, [ 'Включить' ]),
				E('button', {
					'class': 'btn cbi-button cbi-button-negative',
					disabled: readonly || !bot.running || null,
					click: ui.createHandlerFn(this, 'handleBot', 'off')
				}, [ 'Выключить' ]),
				E('button', {
					'class': 'btn cbi-button cbi-button-action',
					disabled: readonly || !bot.running || null,
					click: ui.createHandlerFn(this, 'handleBot', 'restart')
				}, [ 'Перезапустить' ])
			]);

		var vpnCard;
		if (!vpn.installed) {
			// Feature detection: /usr/bin/vpn из репо openwrt-vpn может
			// отсутствовать — панель работает и без него.
			vpnCard = card('🔒 VPN', pill('na', 'не установлен'),
				[ 'тумблер /usr/bin/vpn не найден (репо openwrt-vpn)' ], []);
		} else {
			vpnCard = card('🔒 VPN (Xray + TPROXY)',
				vpn.up ? pill('on', 'включён') : pill('off', 'выключен'),
				[ vpn.up ? 'socks 127.0.0.1:10808 слушает · LAN через VPN' : 'LAN ходит напрямую через ISP' ],
				[
					E('button', {
						'class': 'btn cbi-button cbi-button-positive',
						disabled: readonly || vpn.up || null,
						click: ui.createHandlerFn(this, 'handleVpn', 'on')
					}, [ 'Включить' ]),
					E('button', {
						'class': 'btn cbi-button cbi-button-negative',
						disabled: readonly || !vpn.up || null,
						click: ui.createHandlerFn(this, 'handleVpn', 'off')
					}, [ 'Выключить' ])
				]);
		}

		var tgPill, tgText;
		if (!bot.running) {
			tgPill = pill('off', 'бот выключен');
			tgText = '—';
		} else if (!api.up) {
			tgPill = pill('na', 'нет данных');
			tgText = 'HTTP API выключен — включи HTTP_ADDR в /etc/openwrt-bot/env';
		} else if (st.telegram === 'connected') {
			tgPill = pill('on', 'активен');
			tgText = 'long-poll работает, команды принимаются';
		} else if (vpn.installed && !vpn.up) {
			tgPill = pill('wait', 'ждёт VPN');
			tgText = 'Telegram недоступен без VPN — бот дозвонится сам после «vpn on»';
		} else {
			tgPill = pill('wait', 'подключается…');
			tgText = 'бот пытается дозвониться до Telegram (backoff)';
		}
		var tgCard = card('✈️ Telegram', tgPill, [ tgText ], []);

		return E('div', { style: 'display:flex; gap:1em; flex-wrap:wrap' }, [ botCard, vpnCard, tgCard ]);
	},

	renderDevices: function(st, devs) {
		var bot = (st || {}).bot || {};

		if (!devs || devs.error || !Array.isArray(devs.devices)) {
			var hint, actions = [];
			if (!bot.running) {
				hint = 'Бот выключен — списком устройств управляет он.';
				actions.push(E('button', {
					'class': 'btn cbi-button cbi-button-positive',
					disabled: !L.hasViewPermission() || null,
					click: ui.createHandlerFn(this, 'handleBot', 'on')
				}, [ 'Включить бота' ]));
			} else {
				hint = 'HTTP API бота недоступен (' + (devs && devs.error || '?') +
					') — проверь HTTP_ADDR=127.0.0.1:8787 в /etc/openwrt-bot/env и сделай bot restart.';
			}
			return E('div', { 'class': 'alert-message warning' }, [
				E('p', {}, [ hint ]), E('div', {}, actions)
			]);
		}

		if (devs.devices.length === 0)
			return E('p', { 'class': 'center' }, [ E('em', {}, [ 'LAN пуст' ]) ]);

		var self = this;
		var rows = devs.devices.map(function(d) { return self.renderDeviceRow(d); });

		return E('table', { 'class': 'table' }, [
			E('tr', { 'class': 'tr table-titles' }, [
				E('th', { 'class': 'th' }, [ 'Устройство' ]),
				E('th', { 'class': 'th' }, [ 'IP' ]),
				E('th', { 'class': 'th' }, [ 'MAC' ]),
				E('th', { 'class': 'th' }, [ 'Статус' ]),
				E('th', { 'class': 'th' }, [ 'Лимит, КБ/с' ]),
				E('th', { 'class': 'th' }, [ 'Действия' ])
			])
		].concat(rows));
	},

	renderDeviceRow: function(d) {
		var readonly = !L.hasViewPermission() || null;
		var chips = [];
		if (d.banned)
			chips.push(chip('🚫 бан'));
		if (d.limit_kbytes_per_sec > 0)
			chips.push(chip('⏱ ' + d.limit_kbytes_per_sec + ' КБ/с'));
		if (d.direct)
			chips.push(chip('🌐 мимо VPN'));
		if (chips.length === 0)
			chips.push('—');

		return E('tr', { 'class': 'tr' }, [
			E('td', { 'class': 'td' }, [ E('strong', {}, [ d.hostname || 'без имени' ]) ]),
			E('td', { 'class': 'td' }, [ d.ip || '—' ]),
			E('td', { 'class': 'td' }, [ E('code', {}, [ d.mac ]) ]),
			E('td', { 'class': 'td' }, chips),
			E('td', { 'class': 'td' }, [ this.renderLimitSelect(d) ]),
			E('td', { 'class': 'td', style: 'white-space:nowrap' }, [
				E('button', {
					'class': 'btn cbi-button ' + (d.banned ? 'cbi-button-positive' : 'cbi-button-negative'),
					style: 'margin-right:0.4em',
					disabled: readonly,
					click: ui.createHandlerFn(this, 'handleDeviceAction', d.mac,
						d.banned ? 'unban' : 'ban',
						(d.banned ? '🟢 разбанен: ' : '🚫 забанен: ') + d.mac)
				}, [ d.banned ? 'Разбанить' : 'Забанить' ]),
				E('button', {
					'class': 'btn cbi-button cbi-button-action',
					disabled: readonly,
					click: ui.createHandlerFn(this, 'handleDeviceAction', d.mac,
						d.direct ? 'vpnon' : 'vpnoff',
						(d.direct ? '🔒 снова через VPN: ' : '🌐 мимо VPN: ') + d.mac)
				}, [ d.direct ? 'В VPN' : 'Мимо VPN' ])
			])
		]);
	},

	renderLimitSelect: function(d) {
		var cur = d.limit_kbytes_per_sec || 0;
		var values = LIMIT_PRESETS.slice();
		if (cur > 0 && values.indexOf(cur) < 0) {
			values.push(cur);
			values.sort(function(a, b) { return a - b });
		}

		var opts = [ E('option', { value: '0', selected: cur === 0 ? '' : null }, [ '— нет —' ]) ];
		values.forEach(function(v) {
			opts.push(E('option', { value: String(v), selected: cur === v ? '' : null }, [ String(v) ]));
		});
		opts.push(E('option', { value: 'custom' }, [ 'другой…' ]));

		return E('select', {
			'class': 'cbi-input-select',
			style: 'min-width:7em',
			disabled: !L.hasViewPermission() || null,
			change: ui.createHandlerFn(this, 'handleLimitChange', d)
		}, opts);
	},

	handleLimitChange: function(d, ev) {
		var v = ev.target.value;
		if (v === 'custom') {
			v = window.prompt('Лимит, КБайт/с (1..1000000):', d.limit_kbytes_per_sec || 512);
			if (v === null)
				return this.refresh(); // отмена — вернуть select к текущему значению
		}
		var n = parseInt(v, 10);
		if (isNaN(n) || n < 0 || n > 1000000) {
			ui.addNotification(null, E('p', {}, [ 'Лимит — целое число 1..1000000 КБ/с' ]), 'danger');
			return this.refresh();
		}
		if (n === 0)
			return callDeviceAction(d.mac, 'unlimit').then(this.notify('♾ лимит снят: ' + d.mac));
		return callSetLimit(d.mac, n).then(this.notify('⏱ лимит ' + n + ' КБ/с: ' + d.mac));
	}
});

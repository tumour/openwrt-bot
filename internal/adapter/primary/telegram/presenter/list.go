package presenter

import (
	"fmt"
	"html"

	"github.com/tumour/openwrt-bot/internal/app/device"
)

// Список устройств теперь интерактивный: текст сообщения — только заголовок,
// сами устройства — inline-кнопки (label из DeviceLabel), а детали с копируемым
// MAC живут в карточке (DeviceCard), открывающейся по тапу.

// ListHeader — текст сообщения над кнопками устройств (HTML).
func ListHeader(n int) string {
	if n == 0 {
		return "<i>LAN пуст</i>"
	}
	return fmt.Sprintf("<b>Устройства в LAN (%d)</b>\nНажми на устройство — действия в карточке.", n)
}

// DeviceLabel — подпись inline-кнопки устройства. Plain text: Telegram не
// рендерит HTML в кнопках, поэтому без экранирования и <code>.
func DeviceLabel(v device.View) string {
	return fmt.Sprintf("%s %s · %s", deviceIcon(v), hostOrDefault(v), ipOrDash(v))
}

// DeviceCard — карточка устройства (HTML): полные данные + статусы.
// MAC — <code>-спан, тап по нему копирует. Rate — value object (только цифры),
// не экранируем, как и MAC.
func DeviceCard(v device.View) string {
	ban := "нет"
	if v.Banned {
		ban = "🚫 забанен"
	}
	vpn := "🔒 через VPN"
	if v.Direct {
		vpn = "🌐 напрямую, мимо VPN"
	}
	limit := "нет"
	if !v.Limit.IsZero() {
		limit = fmt.Sprintf("⏱ %s КБ/с", v.Limit)
	}
	return fmt.Sprintf(
		"<b>%s %s</b>\nIP: %s\nMAC: <code>%s</code>\n\nБан: %s\nVPN: %s\nЛимит: %s",
		deviceIcon(v), html.EscapeString(hostOrDefault(v)), html.EscapeString(ipOrDash(v)),
		html.EscapeString(v.Device.MAC.String()), ban, vpn, limit)
}

// CardGone — карточка устройства, пропавшего из DHCP-лиз между /list и тапом.
func CardGone(mac string) string {
	return fmt.Sprintf("устройство <code>%s</code> пропало из DHCP-лиз — обнови список", html.EscapeString(mac))
}

// deviceIcon: бан перекрывает всё (забаненный дропается в prerouting раньше
// остальных правил); лимит показываем прежде vpn-обхода — это более
// «жалобогенное» состояние («почему тормозит?» задают чаще, чем «как я хожу»).
func deviceIcon(v device.View) string {
	switch {
	case v.Banned:
		return "🚫"
	case !v.Limit.IsZero():
		return "⏱"
	case v.Direct:
		return "🌐"
	default:
		return "📱"
	}
}

func hostOrDefault(v device.View) string {
	if v.Device.Hostname == "" {
		return "без имени"
	}
	return v.Device.Hostname
}

func ipOrDash(v device.View) string {
	// len, не nil-check: String() и у пустого (len 0, но не nil) net.IP
	// возвращает "<nil>" — та же ловушка stdlib, что закрыта в httpapi.ipString.
	if len(v.Device.IP) == 0 {
		return "—"
	}
	return v.Device.IP.String()
}

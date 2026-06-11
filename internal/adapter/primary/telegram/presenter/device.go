package presenter

import (
	"fmt"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// MAC в <code> без экранирования: value object гарантирует чарсет
// [0-9a-f:], в нём нечего экранировать.

// Banned форматирует подтверждение бана (HTML). Application rule "повторный бан
// = no-op" означает, что одинаковое сообщение приходит и для первого бана, и для
// повторного — со стороны юзера разницы нет, и это правильно.
func Banned(mac domain.MAC) string {
	return fmt.Sprintf("🔴 устройство <code>%s</code> забанено", mac)
}

func Unbanned(mac domain.MAC) string {
	return fmt.Sprintf("🟢 устройство <code>%s</code> разбанено", mac)
}

// VPNOff/VPNOn — подтверждения vpn-обхода. Как и с баном, no-op на повторе
// даёт то же сообщение — для юзера состояние одинаковое.
func VPNOff(mac domain.MAC) string {
	return fmt.Sprintf("🌐 устройство <code>%s</code> ходит напрямую, без VPN", mac)
}

func VPNOn(mac domain.MAC) string {
	return fmt.Sprintf("🔒 устройство <code>%s</code> снова через VPN", mac)
}

package presenter

import (
	"fmt"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// Banned форматирует подтверждение бана. Application rule "повторный бан = no-op"
// означает, что одинаковое сообщение приходит и для первого бана, и для повторного —
// со стороны юзера разницы нет, и это правильно.
func Banned(mac domain.MAC) string {
	return fmt.Sprintf("🔴 устройство `%s` забанено", mac)
}

func Unbanned(mac domain.MAC) string {
	return fmt.Sprintf("🟢 устройство `%s` разбанено", mac)
}

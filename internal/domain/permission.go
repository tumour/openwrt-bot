package domain

import (
	"fmt"
	"sort"
)

// Permission — атомарное право в боте. Каталог прав — КОД, а не данные:
// право осмысленно только тогда, когда какой-то код его проверяет, поэтому
// новое право появляется тем же коммитом, что и проверка. Какие права у
// какой роли — наоборот, данные (см. Role): админ переназначает их в
// рантайме без деплоя.
type Permission string

// Каталог прав. Право закрывает и действие, и его отображение: нет права —
// нет кнопки/пункта меню (правило «нет пермишена = нет кнопки»).
const (
	PermViewStatus   Permission = "view_status"   // /status: uptime, память, температура
	PermListDevices  Permission = "list_devices"  // /list: устройства в сети
	PermRunSpeedtest Permission = "run_speedtest" // /speedtest
	PermBanDevices   Permission = "ban_devices"   // бан/разбан — «выключить интернет» устройству
	PermManageVPN    Permission = "manage_vpn"    // vpn-обход per-device on/off
	PermManageUsers  Permission = "manage_users"  // список юзеров, approve/reject заявок, роли, удаление
)

// knownPermissions — весь каталог. Пополняется вместе с кодом, который
// проверяет новое право.
var knownPermissions = map[Permission]struct{}{
	PermViewStatus:   {},
	PermListDevices:  {},
	PermRunSpeedtest: {},
	PermBanDevices:   {},
	PermManageVPN:    {},
	PermManageUsers:  {},
}

// NewPermission валидирует право по каталогу. Ошибка ловит, например,
// опечатку в отредактированном руками roles.json.
func NewPermission(s string) (Permission, error) {
	p := Permission(s)
	if _, ok := knownPermissions[p]; !ok {
		return "", fmt.Errorf("%w: %q", ErrUnknownPermission, s)
	}
	return p, nil
}

// AllPermissions — полный каталог в детерминированном порядке. Нужен политике
// «встроенная роль admin догоняет каталог при старте» (см. app/access.Seed).
func AllPermissions() []Permission {
	all := make([]Permission, 0, len(knownPermissions))
	for p := range knownPermissions {
		all = append(all, p)
	}
	sort.Slice(all, func(i, j int) bool { return all[i] < all[j] })
	return all
}

// String реализует fmt.Stringer.
func (p Permission) String() string { return string(p) }

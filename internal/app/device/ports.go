package device

import (
	"context"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// Порты, которые потребляет feature `device`. Хранятся именно здесь
// ("define interfaces at point of use"): adapter/secondary/nftables импортирует
// этот пакет и реализует MACSetPort, но сам пакет device ничего не знает о nftables.

// MACSetPort управляет одним nftables-сетом MAC-адресов. Экземпляров два —
// бан-лист (banned_macs) и vpn-обход (vpn_direct_macs); какой сет получает
// какой use case, решает composition root. Add для уже состоящего MAC возвращает
// domain.ErrAlreadyInSet, Remove для отсутствующего — domain.ErrNotInSet.
type MACSetPort interface {
	Add(ctx context.Context, mac domain.MAC) error
	Remove(ctx context.Context, mac domain.MAC) error
	List(ctx context.Context) ([]domain.MAC, error)
}

// DhcpPort отдаёт текущих DHCP-клиентов LAN.
type DhcpPort interface {
	ListLeases(ctx context.Context) ([]domain.Device, error)
}

// RateLimitPort управляет per-device лимитами скорости (nft limit-объекты +
// map'ы в netdev-таблице). Контракт СОЗНАТЕЛЬНО идемпотентный — контраст с
// типизированным MACSetPort: Set = создать-или-обновить, Remove отсутствующего
// лимита = no-op уже в адаптере. Идемпотентность здесь даёт сам механизм
// (nft destroy = delete-if-exists), а типизированная ошибка только у Remove
// при create-or-update Set была бы асимметрией внутри одного порта.
type RateLimitPort interface {
	Set(ctx context.Context, mac domain.MAC, rate domain.Rate) error
	Remove(ctx context.Context, mac domain.MAC) error
	List(ctx context.Context) (map[domain.MAC]domain.Rate, error)
}

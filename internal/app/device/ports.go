package device

import (
	"context"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// Порты, которые потребляет feature `device`. Хранятся именно здесь
// ("define interfaces at point of use"): adapter/secondary/nftables импортирует
// этот пакет и реализует NftPort, но сам пакет device ничего не знает о nftables.

// NftPort управляет nftables-сетом забаненных MAC-адресов.
type NftPort interface {
	AddBanned(ctx context.Context, mac domain.MAC) error
	RemoveBanned(ctx context.Context, mac domain.MAC) error
	ListBanned(ctx context.Context) ([]domain.MAC, error)
}

// DhcpPort отдаёт текущих DHCP-клиентов LAN.
type DhcpPort interface {
	ListLeases(ctx context.Context) ([]domain.Device, error)
}

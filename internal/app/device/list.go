package device

import (
	"context"
	"fmt"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// List — use case "список всех активных устройств с пометкой забаненных".
// Объединяет данные из двух портов (DhcpPort + NftPort) — типичная orchestration
// на уровне app: один use case дёргает несколько портов и склеивает результат.
type List struct {
	dhcp DhcpPort
	nft  NftPort
}

func NewList(dhcp DhcpPort, nft NftPort) *List {
	return &List{dhcp: dhcp, nft: nft}
}

// View — DTO use-case-уровня. Расширяет domain.Device техническим флагом "забанен".
// В domain не идёт, потому что "забанен" — состояние внешней системы (nftables),
// а не свойство устройства само по себе.
type View struct {
	Device domain.Device
	Banned bool
}

type (
	ListInput  struct{}
	ListOutput struct {
		Devices []View
	}
)

func (uc *List) Execute(ctx context.Context, _ ListInput) (ListOutput, error) {
	leases, err := uc.dhcp.ListLeases(ctx)
	if err != nil {
		return ListOutput{}, fmt.Errorf("list dhcp leases: %w", err)
	}
	banned, err := uc.nft.ListBanned(ctx)
	if err != nil {
		return ListOutput{}, fmt.Errorf("list banned: %w", err)
	}

	bannedSet := make(map[domain.MAC]struct{}, len(banned))
	for _, m := range banned {
		bannedSet[m] = struct{}{}
	}

	views := make([]View, 0, len(leases))
	for _, d := range leases {
		_, isBanned := bannedSet[d.MAC]
		views = append(views, View{Device: d, Banned: isBanned})
	}
	return ListOutput{Devices: views}, nil
}

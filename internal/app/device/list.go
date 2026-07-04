package device

import (
	"context"
	"fmt"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// List — use case "список всех активных устройств с пометками бана, vpn-обхода
// и лимита скорости". Объединяет данные из четырёх портов — типичная
// orchestration на уровне app: один use case дёргает несколько портов и
// склеивает результат.
type List struct {
	dhcp   DhcpPort
	banned MACSetPort
	direct MACSetPort
	limits RateLimitPort
}

func NewList(dhcp DhcpPort, banned, direct MACSetPort, limits RateLimitPort) *List {
	return &List{dhcp: dhcp, banned: banned, direct: direct, limits: limits}
}

// View — DTO use-case-уровня. Расширяет domain.Device техническими флагами
// "забанен" и "ходит мимо VPN". В domain не идут, потому что это состояние
// внешних систем (nftables), а не свойство устройства само по себе.
type View struct {
	Device domain.Device
	Banned bool
	Direct bool        // мимо VPN (в сете vpn_direct_macs)
	Limit  domain.Rate // лимит скорости; zero value = лимита нет (Rate.IsZero)
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
	banned, err := uc.banned.List(ctx)
	if err != nil {
		return ListOutput{}, fmt.Errorf("list banned: %w", err)
	}
	direct, err := uc.direct.List(ctx)
	if err != nil {
		return ListOutput{}, fmt.Errorf("list vpn-direct: %w", err)
	}
	// Лимиты — best-effort (прецедент: температура в /status): на роутере со
	// старым init.d netdev-таблицы нет, но /list с баном и VPN обязан жить.
	// Ошибка не теряется навсегда — всплывёт на /limit, где она уместна.
	limits, err := uc.limits.List(ctx)
	if err != nil {
		limits = nil // отсутствие ключа в nil-map даёт zero value — «лимита нет»
	}

	views := make([]View, 0, len(leases))
	for _, d := range leases {
		views = append(views, View{
			Device: d,
			Banned: macInSet(banned, d.MAC),
			Direct: macInSet(direct, d.MAC),
			Limit:  limits[d.MAC],
		})
	}
	return ListOutput{Devices: views}, nil
}

func macInSet(set []domain.MAC, mac domain.MAC) bool {
	for _, m := range set {
		if m == mac {
			return true
		}
	}
	return false
}

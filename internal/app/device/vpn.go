package device

import (
	"context"
	"errors"
	"fmt"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// DisableVPN — use case "пустить устройство мимо VPN" (/vpnoff). MAC попадает
// в сет vpn_direct_macs: трафик устройства метится 0xff в prerouting до TPROXY,
// xray-цепочка по этой метке делает return — пакет уходит напрямую через ISP.
// Симметричен Ban: тот же порт-паттерн, тот же no-op на повторе.
type DisableVPN struct {
	direct MACSetPort
}

func NewDisableVPN(direct MACSetPort) *DisableVPN {
	return &DisableVPN{direct: direct}
}

type (
	DisableVPNInput struct {
		MAC domain.MAC
	}
	DisableVPNOutput struct{}
)

func (uc *DisableVPN) Execute(ctx context.Context, in DisableVPNInput) (DisableVPNOutput, error) {
	if err := uc.direct.Add(ctx, in.MAC); err != nil {
		if errors.Is(err, domain.ErrAlreadyInSet) {
			return DisableVPNOutput{}, nil
		}
		return DisableVPNOutput{}, fmt.Errorf("vpn off %s: %w", in.MAC, err)
	}
	return DisableVPNOutput{}, nil
}

// EnableVPN — use case "вернуть устройство в VPN" (/vpnon): убрать MAC из
// сета обхода. Отсутствие в сете = no-op (устройство и так в VPN).
type EnableVPN struct {
	direct MACSetPort
}

func NewEnableVPN(direct MACSetPort) *EnableVPN {
	return &EnableVPN{direct: direct}
}

type (
	EnableVPNInput struct {
		MAC domain.MAC
	}
	EnableVPNOutput struct{}
)

func (uc *EnableVPN) Execute(ctx context.Context, in EnableVPNInput) (EnableVPNOutput, error) {
	if err := uc.direct.Remove(ctx, in.MAC); err != nil {
		if errors.Is(err, domain.ErrNotInSet) {
			return EnableVPNOutput{}, nil
		}
		return EnableVPNOutput{}, fmt.Errorf("vpn on %s: %w", in.MAC, err)
	}
	return EnableVPNOutput{}, nil
}

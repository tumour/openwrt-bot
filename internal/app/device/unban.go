package device

import (
	"context"
	"errors"
	"fmt"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// Unban — use case "разбанить устройство".
type Unban struct {
	nft NftPort
}

func NewUnban(nft NftPort) *Unban {
	return &Unban{nft: nft}
}

type (
	UnbanInput struct {
		MAC domain.MAC
	}
	UnbanOutput struct{}
)

func (uc *Unban) Execute(ctx context.Context, in UnbanInput) (UnbanOutput, error) {
	if err := uc.nft.RemoveBanned(ctx, in.MAC); err != nil {
		if errors.Is(err, domain.ErrNotBanned) {
			return UnbanOutput{}, nil
		}
		return UnbanOutput{}, fmt.Errorf("unban %s: %w", in.MAC, err)
	}
	return UnbanOutput{}, nil
}

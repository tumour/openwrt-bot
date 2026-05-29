package device

import (
	"context"
	"errors"
	"fmt"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// Ban — use case "забанить устройство по MAC".
type Ban struct {
	nft NftPort
}

func NewBan(nft NftPort) *Ban {
	return &Ban{nft: nft}
}

type (
	BanInput struct {
		MAC domain.MAC
	}
	BanOutput struct{}
)

// Execute — application rule: повторный бан уже забаненного MAC = no-op (не ошибка
// для вызывающего). Это решение workflow-уровня, потому в app/, не в domain/.
func (uc *Ban) Execute(ctx context.Context, in BanInput) (BanOutput, error) {
	if err := uc.nft.AddBanned(ctx, in.MAC); err != nil {
		if errors.Is(err, domain.ErrAlreadyBanned) {
			return BanOutput{}, nil
		}
		return BanOutput{}, fmt.Errorf("ban %s: %w", in.MAC, err)
	}
	return BanOutput{}, nil
}

package device

import (
	"context"
	"fmt"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// SetLimit — use case "ограничить скорость устройства" (/limit). Одно значение
// N КБ/с режет каждое направление (download и upload) независимо. Порт
// идемпотентен (создать-или-обновить), поэтому в отличие от Ban здесь нечего
// "проглатывать" — use case только делегирует и оборачивает ошибку.
type SetLimit struct {
	limits RateLimitPort
}

func NewSetLimit(limits RateLimitPort) *SetLimit {
	return &SetLimit{limits: limits}
}

type (
	SetLimitInput struct {
		MAC  domain.MAC
		Rate domain.Rate
	}
	SetLimitOutput struct{}
)

func (uc *SetLimit) Execute(ctx context.Context, in SetLimitInput) (SetLimitOutput, error) {
	if err := uc.limits.Set(ctx, in.MAC, in.Rate); err != nil {
		return SetLimitOutput{}, fmt.Errorf("limit %s: %w", in.MAC, err)
	}
	return SetLimitOutput{}, nil
}

// RemoveLimit — use case "снять лимит скорости" (/unlimit). Снятие
// отсутствующего лимита — no-op на уровне порта (идемпотентный контракт).
type RemoveLimit struct {
	limits RateLimitPort
}

func NewRemoveLimit(limits RateLimitPort) *RemoveLimit {
	return &RemoveLimit{limits: limits}
}

type (
	RemoveLimitInput struct {
		MAC domain.MAC
	}
	RemoveLimitOutput struct{}
)

func (uc *RemoveLimit) Execute(ctx context.Context, in RemoveLimitInput) (RemoveLimitOutput, error) {
	if err := uc.limits.Remove(ctx, in.MAC); err != nil {
		return RemoveLimitOutput{}, fmt.Errorf("unlimit %s: %w", in.MAC, err)
	}
	return RemoveLimitOutput{}, nil
}

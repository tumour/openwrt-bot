package status

import (
	"context"
	"fmt"
)

// GetStatus — use case "получить статус роутера". Тонкий use case (фактически
// прокси к одному порту) — это нормально, главное чтобы он существовал отдельной
// единицей: тогда добавить рядом валидацию / форматирование / кеширование можно
// без правки adapter-кода.
type GetStatus struct {
	sys SystemPort
}

func NewGetStatus(sys SystemPort) *GetStatus {
	return &GetStatus{sys: sys}
}

type (
	GetStatusInput  struct{}
	GetStatusOutput struct {
		Snapshot Snapshot
	}
)

func (uc *GetStatus) Execute(ctx context.Context, _ GetStatusInput) (GetStatusOutput, error) {
	s, err := uc.sys.Snapshot(ctx)
	if err != nil {
		return GetStatusOutput{}, fmt.Errorf("get system snapshot: %w", err)
	}
	return GetStatusOutput{Snapshot: s}, nil
}

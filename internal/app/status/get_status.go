package status

import (
	"context"
	"fmt"
)

// GetStatus — use case "получить статус роутера". Оркестрирует два порта:
// SystemPort (uptime/load/memory через ubus) и ThermalPort (температура из sysfs).
// Держать их раздельно honest — у них разные источники данных; склейка живёт здесь.
type GetStatus struct {
	sys     SystemPort
	thermal ThermalPort
}

func NewGetStatus(sys SystemPort, thermal ThermalPort) *GetStatus {
	return &GetStatus{sys: sys, thermal: thermal}
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

	// Температура — необязательное обогащение: датчика может не быть на этом железе
	// (см. Snapshot.TempCelsius). Ошибку чтения сознательно НЕ пробрасываем — одна
	// декоративная метрика не должна ронять весь /status. Нет температуры → остаётся
	// 0, и presenter её скрывает.
	if temp, err := uc.thermal.Temperature(ctx); err == nil {
		s.TempCelsius = temp
	}
	return GetStatusOutput{Snapshot: s}, nil
}

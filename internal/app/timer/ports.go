package timer

import (
	"context"
	"time"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// Порты, которые потребляет feature `timer` («define interfaces at point of
// use»). Реализация — adapter/secondary/schedule поверх обобщённого таймерного
// движка platform/scheduler: app по dependency rule не видит platform, поэтому
// контракт выражен в domain-типах, а трансляцию (типы задач, ошибки) делает
// адаптер — как nftables переводит stderr в доменные ошибки.

// SchedulerPort — постановка/просмотр/отмена отложенных device-операций.
// Cancel несуществующей задачи возвращает domain.ErrTaskNotFound (кнопка
// отмены переживает и сам таймер, и рестарт бота — это штатный случай).
type SchedulerPort interface {
	Schedule(ctx context.Context, delay time.Duration, job domain.DeviceJob) (domain.TaskID, error)
	List(ctx context.Context) ([]View, error)
	Cancel(ctx context.Context, id domain.TaskID) error
}

// View — DTO use-case-уровня: запланированная задача с уже посчитанным (от
// часов движка) остатком до срабатывания. Аналог device.View.
type View struct {
	ID        domain.TaskID
	Job       domain.DeviceJob
	FireAt    time.Time
	Remaining time.Duration
}

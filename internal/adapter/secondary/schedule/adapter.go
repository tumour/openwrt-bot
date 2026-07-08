// Package schedule — secondary adapter: реализует timer.SchedulerPort поверх
// обобщённого движка platform/scheduler. Весь смысл — трансляция на границе:
// generic-типы движка ↔ domain-типы порта, инфраструктурная ErrTaskNotFound ↔
// доменная (как nftables переводит stderr в ErrNotInSet). Бизнес-логики нет.
package schedule

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tumour/openwrt-bot/internal/app/timer"
	"github.com/tumour/openwrt-bot/internal/domain"
	"github.com/tumour/openwrt-bot/internal/platform/scheduler"
)

// Adapter — обёртка движка с задачами domain.DeviceJob.
type Adapter struct {
	engine *scheduler.Scheduler[domain.DeviceJob]
}

func NewAdapter(engine *scheduler.Scheduler[domain.DeviceJob]) *Adapter {
	return &Adapter{engine: engine}
}

// Schedule ставит задачу и возвращает её доменный ID.
func (a *Adapter) Schedule(ctx context.Context, delay time.Duration, job domain.DeviceJob) (domain.TaskID, error) {
	task, err := a.engine.Schedule(ctx, delay, job)
	if err != nil {
		return 0, err
	}
	return domain.TaskID(task.ID), nil
}

// List отдаёт активные задачи в терминах app (порядок движка — по FireAt).
func (a *Adapter) List(ctx context.Context) ([]timer.View, error) {
	views, err := a.engine.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]timer.View, 0, len(views))
	for _, v := range views {
		out = append(out, timer.View{
			ID:        domain.TaskID(v.ID),
			Job:       v.Job,
			FireAt:    v.FireAt,
			Remaining: v.Remaining,
		})
	}
	return out, nil
}

// Cancel снимает задачу, типизируя «не нашлось» в доменную ошибку.
func (a *Adapter) Cancel(ctx context.Context, id domain.TaskID) error {
	if err := a.engine.Cancel(ctx, scheduler.TaskID(id)); err != nil {
		if errors.Is(err, scheduler.ErrTaskNotFound) {
			return fmt.Errorf("%s: %w", id, domain.ErrTaskNotFound)
		}
		return err
	}
	return nil
}

// Package timer — вертикальный слайс «отложенные операции над устройствами»:
// поставить действие (бан/разбан/vpn) на выполнение через N минут, посмотреть
// активные, отменить. Сами действия при срабатывании выполняет composition root
// через use cases device.* — здесь только управление расписанием.
//
// Сегодня use cases — тонкая оркестрация одного порта; слой существует как шов:
// персистентность таймеров (пережить рестарт), аудит и роли лягут сюда, не в
// движок и не в handler.
package timer

import (
	"context"
	"fmt"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// Schedule — use case «поставить отложенное действие над устройством».
type Schedule struct {
	sched SchedulerPort
}

func NewSchedule(sched SchedulerPort) *Schedule {
	return &Schedule{sched: sched}
}

type (
	ScheduleInput struct {
		MAC    domain.MAC
		Action domain.Action
		Delay  domain.Minutes
	}
	ScheduleOutput struct {
		ID domain.TaskID
	}
)

func (uc *Schedule) Execute(ctx context.Context, in ScheduleInput) (ScheduleOutput, error) {
	id, err := uc.sched.Schedule(ctx, in.Delay.Duration(), domain.DeviceJob{MAC: in.MAC, Action: in.Action})
	if err != nil {
		return ScheduleOutput{}, fmt.Errorf("schedule %s %s: %w", in.Action, in.MAC, err)
	}
	return ScheduleOutput{ID: id}, nil
}

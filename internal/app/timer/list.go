package timer

import (
	"context"
	"fmt"
)

// List — use case «активные таймеры»: по возрастанию времени срабатывания
// (порядок гарантирует порт), с остатком до него.
type List struct {
	sched SchedulerPort
}

func NewList(sched SchedulerPort) *List {
	return &List{sched: sched}
}

type (
	ListInput  struct{}
	ListOutput struct {
		Tasks []View
	}
)

func (uc *List) Execute(ctx context.Context, _ ListInput) (ListOutput, error) {
	tasks, err := uc.sched.List(ctx)
	if err != nil {
		return ListOutput{}, fmt.Errorf("list timers: %w", err)
	}
	return ListOutput{Tasks: tasks}, nil
}

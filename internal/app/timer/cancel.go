package timer

import (
	"context"
	"fmt"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// Cancel — use case «снять таймер». domain.ErrTaskNotFound пробрасывается
// типизированно (handler делает из него мягкий toast): в отличие от «повторный
// бан = no-op» здесь вызывающему важно знать, что отменять было нечего.
type Cancel struct {
	sched SchedulerPort
}

func NewCancel(sched SchedulerPort) *Cancel {
	return &Cancel{sched: sched}
}

type (
	CancelInput struct {
		ID domain.TaskID
	}
	CancelOutput struct{}
)

func (uc *Cancel) Execute(ctx context.Context, in CancelInput) (CancelOutput, error) {
	if err := uc.sched.Cancel(ctx, in.ID); err != nil {
		return CancelOutput{}, fmt.Errorf("cancel timer %s: %w", in.ID, err)
	}
	return CancelOutput{}, nil
}

package schedule

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tumour/openwrt-bot/internal/domain"
	"github.com/tumour/openwrt-bot/internal/platform/scheduler"
)

// nopRunner — движку нужен Runner, но в тестах адаптера срабатываний нет.
type nopRunner struct{}

func (nopRunner) Run(context.Context, domain.DeviceJob) error { return nil }

// fixedClock — часы с ручным управлением: таймеры здесь никогда не срабатывают,
// тестируется только трансляция типов и ошибок.
type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }
func (c fixedClock) AfterFunc(time.Duration, func()) func() bool {
	return func() bool { return true }
}

func newAdapter() *Adapter {
	clock := fixedClock{now: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	return NewAdapter(scheduler.New[domain.DeviceJob](clock, nopRunner{}))
}

func mustMAC(t *testing.T, s string) domain.MAC {
	t.Helper()
	mac, err := domain.NewMAC(s)
	if err != nil {
		t.Fatalf("NewMAC(%q): %v", s, err)
	}
	return mac
}

func TestAdapter_ScheduleAndList_TranslatesTypes(t *testing.T) {
	a := newAdapter()
	job := domain.DeviceJob{MAC: mustMAC(t, "aa:bb:cc:11:22:33"), Action: domain.ActionBan}

	id, err := a.Schedule(context.Background(), 30*time.Minute, job)
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	views, err := a.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("List len = %d, want 1", len(views))
	}
	v := views[0]
	if v.ID != id || v.Job != job || v.Remaining != 30*time.Minute {
		t.Errorf("view = %+v (id %d)", v, id)
	}
}

func TestAdapter_Cancel_OK(t *testing.T) {
	a := newAdapter()
	id, _ := a.Schedule(context.Background(), time.Minute, domain.DeviceJob{
		MAC: mustMAC(t, "aa:bb:cc:11:22:33"), Action: domain.ActionVPNOn,
	})

	if err := a.Cancel(context.Background(), id); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	views, _ := a.List(context.Background())
	if len(views) != 0 {
		t.Errorf("после Cancel список должен быть пуст, got %d", len(views))
	}
}

// Граница обязана типизировать «не нашлось» в domain.ErrTaskNotFound — по ней
// handler отличает штатную мёртвую кнопку от реальной ошибки.
func TestAdapter_Cancel_Unknown_TranslatesToDomainError(t *testing.T) {
	err := newAdapter().Cancel(context.Background(), domain.TaskID(999))
	if !errors.Is(err, domain.ErrTaskNotFound) {
		t.Errorf("err = %v, want domain.ErrTaskNotFound", err)
	}
	if errors.Is(err, scheduler.ErrTaskNotFound) {
		t.Errorf("инфраструктурная ошибка движка не должна протекать наружу: %v", err)
	}
}

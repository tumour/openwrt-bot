package timer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// stubScheduler — стаб SchedulerPort: пишет аргументы, отдаёт заготовки.
type stubScheduler struct {
	scheduleErr error
	listErr     error
	cancelErr   error

	views []View

	gotDelay  time.Duration
	gotJob    domain.DeviceJob
	gotCancel domain.TaskID
}

func (s *stubScheduler) Schedule(_ context.Context, delay time.Duration, job domain.DeviceJob) (domain.TaskID, error) {
	s.gotDelay, s.gotJob = delay, job
	return 7, s.scheduleErr
}
func (s *stubScheduler) List(_ context.Context) ([]View, error) { return s.views, s.listErr }
func (s *stubScheduler) Cancel(_ context.Context, id domain.TaskID) error {
	s.gotCancel = id
	return s.cancelErr
}

func newMAC(t *testing.T, s string) domain.MAC {
	t.Helper()
	mac, err := domain.NewMAC(s)
	if err != nil {
		t.Fatalf("NewMAC(%q): %v", s, err)
	}
	return mac
}

func TestSchedule_Execute_OK(t *testing.T) {
	port := &stubScheduler{}
	uc := NewSchedule(port)
	mac := newMAC(t, "aa:bb:cc:11:22:33")
	mins, _ := domain.NewMinutes(45)

	out, err := uc.Execute(context.Background(), ScheduleInput{MAC: mac, Action: domain.ActionBan, Delay: mins})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.ID != 7 {
		t.Errorf("ID = %d, want 7", out.ID)
	}
	if port.gotDelay != 45*time.Minute {
		t.Errorf("delay = %v, want 45m", port.gotDelay)
	}
	if port.gotJob != (domain.DeviceJob{MAC: mac, Action: domain.ActionBan}) {
		t.Errorf("job = %+v", port.gotJob)
	}
}

func TestSchedule_Execute_Error_Propagates(t *testing.T) {
	boom := errors.New("scheduler down")
	uc := NewSchedule(&stubScheduler{scheduleErr: boom})
	mins, _ := domain.NewMinutes(5)

	_, err := uc.Execute(context.Background(), ScheduleInput{MAC: newMAC(t, "aa:bb:cc:11:22:33"), Action: domain.ActionUnban, Delay: mins})
	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want wrapped %v", err, boom)
	}
}

func TestList_Execute_OK(t *testing.T) {
	want := []View{{ID: 1, Job: domain.DeviceJob{Action: domain.ActionVPNOff}, Remaining: time.Minute}}
	uc := NewList(&stubScheduler{views: want})

	out, err := uc.Execute(context.Background(), ListInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Tasks) != 1 || out.Tasks[0].ID != 1 {
		t.Errorf("tasks = %+v, want %+v", out.Tasks, want)
	}
}

func TestList_Execute_Error_Propagates(t *testing.T) {
	boom := errors.New("boom")
	uc := NewList(&stubScheduler{listErr: boom})

	if _, err := uc.Execute(context.Background(), ListInput{}); !errors.Is(err, boom) {
		t.Errorf("err = %v, want wrapped %v", err, boom)
	}
}

func TestCancel_Execute_OK(t *testing.T) {
	port := &stubScheduler{}
	uc := NewCancel(port)

	if _, err := uc.Execute(context.Background(), CancelInput{ID: 42}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port.gotCancel != 42 {
		t.Errorf("cancel id = %d, want 42", port.gotCancel)
	}
}

// ErrTaskNotFound пробрасывается типизированно: handler по нему показывает
// мягкий toast «уже неактивен», а не ошибку.
func TestCancel_Execute_NotFound_Typed(t *testing.T) {
	uc := NewCancel(&stubScheduler{cancelErr: domain.ErrTaskNotFound})

	_, err := uc.Execute(context.Background(), CancelInput{ID: 42})
	if !errors.Is(err, domain.ErrTaskNotFound) {
		t.Errorf("err = %v, want ErrTaskNotFound", err)
	}
}

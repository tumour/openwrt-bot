package scheduler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// testJob — произвольный тип задачи: движок обобщён, домен ему не нужен.
type testJob struct{ name string }

// fakeRunner фиксирует выполненные задачи (потокобезопасно: fire может звать из
// разных горутин).
type fakeRunner struct {
	mu  sync.Mutex
	ran []testJob
	err error
}

func (r *fakeRunner) Run(_ context.Context, job testJob) error {
	r.mu.Lock()
	r.ran = append(r.ran, job)
	r.mu.Unlock()
	return r.err
}

func (r *fakeRunner) calls() []testJob {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]testJob(nil), r.ran...)
}

// fakeClock — детерминированные часы: таймеры «срабатывают» из теста через
// advance, без реального ожидания.
type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*fakeTimer
}

type fakeTimer struct {
	due     time.Time
	fire    func()
	stopped bool
	fired   bool
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) AfterFunc(d time.Duration, fire func()) func() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &fakeTimer{due: c.now.Add(d), fire: fire}
	c.timers = append(c.timers, t)
	return func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		if t.stopped || t.fired {
			return false
		}
		t.stopped = true
		return true
	}
}

// advance двигает часы на d и синхронно вызывает fire() у наступивших таймеров.
func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	var due []*fakeTimer
	for _, t := range c.timers {
		if !t.stopped && !t.fired && !t.due.After(c.now) {
			t.fired = true
			due = append(due, t)
		}
	}
	c.mu.Unlock()
	for _, t := range due {
		t.fire()
	}
}

func TestTaskID_String(t *testing.T) {
	if got := TaskID(42).String(); got != "42" {
		t.Errorf("TaskID(42).String() = %q, want %q", got, "42")
	}
}

func TestSchedule_ListsPendingTask(t *testing.T) {
	clk := newFakeClock()
	s := New[testJob](clk, &fakeRunner{})

	task, err := s.Schedule(context.Background(), 30*time.Minute, testJob{"a"})
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	views, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("List len = %d, want 1", len(views))
	}
	if views[0].ID != task.ID {
		t.Errorf("ID = %d, want %d", views[0].ID, task.ID)
	}
	if views[0].Remaining != 30*time.Minute {
		t.Errorf("Remaining = %v, want %v", views[0].Remaining, 30*time.Minute)
	}
	if !views[0].FireAt.Equal(clk.now.Add(30 * time.Minute)) {
		t.Errorf("FireAt = %v, want %v", views[0].FireAt, clk.now.Add(30*time.Minute))
	}
}

func TestRemaining_ShrinksAsTimePasses(t *testing.T) {
	clk := newFakeClock()
	s := New[testJob](clk, &fakeRunner{})
	if _, err := s.Schedule(context.Background(), 10*time.Minute, testJob{"a"}); err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	clk.advance(3 * time.Minute) // 3 мин ещё не срабатывают (due 10 мин)

	views, _ := s.List(context.Background())
	if len(views) != 1 {
		t.Fatalf("task should still be pending, got %d", len(views))
	}
	if views[0].Remaining != 7*time.Minute {
		t.Errorf("Remaining = %v, want %v", views[0].Remaining, 7*time.Minute)
	}
}

func TestFire_RunsJobAndRemoves(t *testing.T) {
	clk := newFakeClock()
	runner := &fakeRunner{}
	s := New[testJob](clk, runner)
	if _, err := s.Schedule(context.Background(), 5*time.Minute, testJob{"boom"}); err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	clk.advance(5 * time.Minute)

	calls := runner.calls()
	if len(calls) != 1 || calls[0].name != "boom" {
		t.Fatalf("runner calls = %v, want [boom]", calls)
	}
	views, _ := s.List(context.Background())
	if len(views) != 0 {
		t.Errorf("fired task must be removed, got %d", len(views))
	}
}

func TestFire_RunnerErrorDoesNotPanic(t *testing.T) {
	clk := newFakeClock()
	runner := &fakeRunner{err: errors.New("nft boom")}
	s := New[testJob](clk, runner)
	if _, err := s.Schedule(context.Background(), time.Minute, testJob{"x"}); err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	clk.advance(time.Minute) // не должно паниковать; задача всё равно снимается

	if len(runner.calls()) != 1 {
		t.Errorf("runner should have been called once")
	}
	views, _ := s.List(context.Background())
	if len(views) != 0 {
		t.Errorf("failed task must still be removed, got %d", len(views))
	}
}

func TestCancel_StopsAndRemoves(t *testing.T) {
	clk := newFakeClock()
	runner := &fakeRunner{}
	s := New[testJob](clk, runner)
	task, _ := s.Schedule(context.Background(), 10*time.Minute, testJob{"a"})

	if err := s.Cancel(context.Background(), task.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	clk.advance(20 * time.Minute) // сработать не должно — таймер снят

	if len(runner.calls()) != 0 {
		t.Errorf("cancelled task must not run, got %v", runner.calls())
	}
	views, _ := s.List(context.Background())
	if len(views) != 0 {
		t.Errorf("cancelled task must be gone, got %d", len(views))
	}
}

func TestCancel_Unknown(t *testing.T) {
	s := New[testJob](newFakeClock(), &fakeRunner{})
	err := s.Cancel(context.Background(), TaskID(999))
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("Cancel unknown err = %v, want ErrTaskNotFound", err)
	}
}

func TestList_SortedByFireAt(t *testing.T) {
	clk := newFakeClock()
	s := New[testJob](clk, &fakeRunner{})
	// ставим не по возрастанию задержки
	s.Schedule(context.Background(), 30*time.Minute, testJob{"mid"})
	s.Schedule(context.Background(), 5*time.Minute, testJob{"soon"})
	s.Schedule(context.Background(), 60*time.Minute, testJob{"late"})

	views, _ := s.List(context.Background())
	got := []string{views[0].Job.name, views[1].Job.name, views[2].Job.name}
	want := []string{"soon", "mid", "late"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

// realClock — настоящие time.AfterFunc-часы для конкурентных тестов: fire
// приходит из таймерных горутин runtime, как в проде. Дубль system.Clock
// сознательный: platform не должен зависеть от adapter даже в тестах.
type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }
func (realClock) AfterFunc(d time.Duration, fire func()) func() bool {
	return time.AfterFunc(d, fire).Stop
}

// countRunner считает выполнения по задаче — ловим двойные срабатывания.
type countRunner struct {
	mu   sync.Mutex
	runs map[string]int
}

func (r *countRunner) Run(_ context.Context, job testJob) error {
	r.mu.Lock()
	r.runs[job.name]++
	r.mu.Unlock()
	return nil
}

// Гонка fire против Cancel на реальных таймерах: каждая задача обязана
// выполниться 0 или 1 раз (кто первым взял мьютекс, тот и решил), -race не
// должен ругаться. Это единственный тест, где fire бежит из чужих горутин
// одновременно с API движка — fakeClock такое не воспроизводит.
func TestConcurrent_FireVsCancel(t *testing.T) {
	const n = 200
	runner := &countRunner{runs: make(map[string]int, n)}
	s := New[testJob](realClock{}, runner)

	tasks := make([]Task[testJob], n)
	for i := range tasks {
		task, err := s.Schedule(context.Background(), time.Millisecond, testJob{name: TaskID(i).String()})
		if err != nil {
			t.Fatalf("Schedule: %v", err)
		}
		tasks[i] = task
	}

	var wg sync.WaitGroup
	for _, task := range tasks {
		wg.Add(1)
		go func(id TaskID) {
			defer wg.Done()
			_ = s.Cancel(context.Background(), id) // проигрыш гонки (ErrTaskNotFound) — норма
		}(task.ID)
	}
	wg.Wait()
	time.Sleep(20 * time.Millisecond) // дать сработать невыигранным таймерам

	runner.mu.Lock()
	defer runner.mu.Unlock()
	for name, c := range runner.runs {
		if c > 1 {
			t.Errorf("задача %s выполнена %d раз — двойное срабатывание", name, c)
		}
	}
	views, _ := s.List(context.Background())
	if len(views) != 0 {
		t.Errorf("после гонки не должно остаться активных, got %d", len(views))
	}
}

// Shutdown на реальных таймерах: Run обязан дождаться in-flight fire и после
// возврата никаких новых выполнений быть не должно.
func TestConcurrent_ShutdownWaitsInFlightFires(t *testing.T) {
	const n = 100
	runner := &countRunner{runs: make(map[string]int, n)}
	s := New[testJob](realClock{}, runner)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()

	for i := 0; i < n; i++ {
		if _, err := s.Schedule(context.Background(), time.Millisecond, testJob{name: TaskID(i).String()}); err != nil {
			t.Fatalf("Schedule: %v", err)
		}
	}
	time.Sleep(time.Millisecond) // часть таймеров успевает сработать — это и нужно
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Run вернулся ⇒ все in-flight завершены; фиксируем счётчики и убеждаемся,
	// что задним числом ничего не выполняется.
	snapshot := func() map[string]int {
		runner.mu.Lock()
		defer runner.mu.Unlock()
		out := make(map[string]int, len(runner.runs))
		for k, v := range runner.runs {
			out[k] = v
		}
		return out
	}
	before := snapshot()
	time.Sleep(20 * time.Millisecond)
	after := snapshot()
	if len(before) != len(after) {
		t.Errorf("после Run выполнились ещё задачи: было %d, стало %d", len(before), len(after))
	}
}

func TestRun_ShutdownStopsTimersAndBlocksScheduling(t *testing.T) {
	clk := newFakeClock()
	runner := &fakeRunner{}
	s := New[testJob](clk, runner)
	s.Schedule(context.Background(), 10*time.Minute, testJob{"pending"})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run returned %v, want nil", err)
	}

	// таймеры сняты — срабатывания нет
	clk.advance(20 * time.Minute)
	if len(runner.calls()) != 0 {
		t.Errorf("shutdown must stop pending timers, got %v", runner.calls())
	}
	// новые задачи не принимаются
	if _, err := s.Schedule(context.Background(), time.Minute, testJob{"late"}); !errors.Is(err, ErrClosed) {
		t.Errorf("Schedule after shutdown err = %v, want ErrClosed", err)
	}
}

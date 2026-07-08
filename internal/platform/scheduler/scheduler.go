// Package scheduler — обобщённый таймерный примитив: «выполнить задачу через
// заданную задержку», со списком активных задач и отменой. Как rungroup — это
// инфраструктура на stdlib, без знания о доменах: тип задачи J подставляет
// вызывающий (composition root), а что делать при срабатывании — инжектируемый
// Runner[J]. Один и тот же движок обслуживает любые отложенные эффекты (бан
// устройства сегодня, отложенное уведомление завтра) — новый тип задачи движок
// не трогает.
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"
)

// fireTimeout — потолок на одно срабатывание: зависший exec не должен держать
// таймерную горутину. С запасом на самый медленный вызов на слабом роутере.
const fireTimeout = 10 * time.Second

var (
	// ErrTaskNotFound — отмена/поиск по несуществующему ID. Штатно случается,
	// когда кнопка пришла из сообщения до рестарта: задачи живут в памяти процесса.
	ErrTaskNotFound = errors.New("scheduled task not found")
	// ErrClosed — планирование после остановки движка (graceful shutdown).
	ErrClosed = errors.New("scheduler is shut down")
)

// TaskID — идентификатор задачи, уникальный в пределах процесса. Присваивает
// планировщик (монотонный счётчик); ходит в payload кнопки отмены.
type TaskID uint64

// String реализует fmt.Stringer.
func (id TaskID) String() string { return strconv.FormatUint(uint64(id), 10) }

// Clock — время как порт: детерминированные тесты + единственный шов к системным
// часам. Реальная реализация — adapter/secondary/system. AfterFunc вызывает fire
// по истечении d в своей горутине; stop отменяет ещё не сработавший таймер
// (true — успел отменить, как у time.Timer.Stop).
type Clock interface {
	Now() time.Time
	AfterFunc(d time.Duration, fire func()) (stop func() bool)
}

// Runner[J] выполняет задачу при срабатывании таймера. Реализацию даёт
// composition root: для device-задач она делегирует use cases device.* —
// правило «повтор = no-op» живёт там и переиспользуется, рассинхрона с кнопками нет.
type Runner[J any] interface {
	Run(ctx context.Context, job J) error
}

// Task[J] — запланированная задача: что (Job) и когда (FireAt).
type Task[J any] struct {
	ID     TaskID
	Job    J
	FireAt time.Time
}

// TaskView[J] — Task с уже посчитанным (от текущих часов) остатком до срабатывания.
// Считать остаток должен движок: только у него есть Clock. Task встроен —
// потребитель читает v.Job/v.FireAt без лишней ступеньки.
type TaskView[J any] struct {
	Task[J]
	Remaining time.Duration
}

// entry — задача плюс runtime-хэндл её таймера (для отмены). Хэндл эфемерен и
// в памяти процесса — вот почему он отдельно от сериализуемых данных Task.
type entry[J any] struct {
	task Task[J]
	stop func() bool
}

// Scheduler[J] — движок: держит активные задачи и их таймеры под мьютексом.
// Zero value непригоден — создавать только через New.
type Scheduler[J any] struct {
	clock Clock
	run   Runner[J]

	mu      sync.Mutex
	baseCtx context.Context // ctx для срабатываний; заменяется в Run на отменяемый
	closed  bool
	lastID  TaskID
	tasks   map[TaskID]*entry[J]
	// firing учитывает выполняющиеся fire: Run на shutdown дожидается их,
	// чтобы процесс не вышел посреди действия (exec и так оборван отменой
	// baseCtx, но завершение должно быть упорядоченным, не гонкой с exit).
	firing sync.WaitGroup
}

// New собирает движок. baseCtx до Run — context.Background: срабатывания в окне
// «до старта Run» всё равно ограничены fireTimeout; Run заменит его на ctx
// приложения, чтобы SIGTERM обрывал exec'и.
func New[J any](clock Clock, run Runner[J]) *Scheduler[J] {
	return &Scheduler[J]{
		clock:   clock,
		run:     run,
		baseCtx: context.Background(),
		tasks:   make(map[TaskID]*entry[J]),
	}
}

// Schedule ставит job на выполнение через delay и возвращает созданную задачу.
// ctx не используется (постановка не блокирует и не ходит в I/O) — присутствует
// ради единообразия сигнатур сервисов.
func (s *Scheduler[J]) Schedule(_ context.Context, delay time.Duration, job J) (Task[J], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Task[J]{}, ErrClosed
	}
	s.lastID++
	id := s.lastID
	task := Task[J]{ID: id, Job: job, FireAt: s.clock.Now().Add(delay)}
	stop := s.clock.AfterFunc(delay, func() { s.fire(id) })
	s.tasks[id] = &entry[J]{task: task, stop: stop}
	return task, nil
}

// fire — колбэк сработавшего таймера (чужая горутина): снять задачу и выполнить.
func (s *Scheduler[J]) fire(id TaskID) {
	s.mu.Lock()
	e, ok := s.tasks[id]
	if !ok { // отменена или движок остановлен — гонку с Cancel/Run решает эта проверка
		s.mu.Unlock()
		return
	}
	delete(s.tasks, id)
	baseCtx := s.baseCtx
	s.firing.Add(1) // под мьютексом: Run видит либо задачу в map, либо счётчик
	s.mu.Unlock()
	defer s.firing.Done()

	ctx, cancel := context.WithTimeout(baseCtx, fireTimeout)
	defer cancel()
	// Исход логирует Runner (composition root); здесь намеренно не трогаем
	// возврат — упавшее действие не должно ронять таймерную горутину.
	_ = s.run.Run(ctx, e.task.Job)
}

// List — активные задачи, отсортированные по времени срабатывания, с остатком
// до него от текущих часов.
func (s *Scheduler[J]) List(_ context.Context) ([]TaskView[J], error) {
	s.mu.Lock()
	now := s.clock.Now()
	views := make([]TaskView[J], 0, len(s.tasks))
	for _, e := range s.tasks {
		views = append(views, TaskView[J]{Task: e.task, Remaining: e.task.FireAt.Sub(now)})
	}
	s.mu.Unlock()

	sort.Slice(views, func(i, j int) bool {
		return views[i].FireAt.Before(views[j].FireAt)
	})
	return views, nil
}

// Cancel снимает задачу по ID. Нет такой — ErrTaskNotFound.
func (s *Scheduler[J]) Cancel(_ context.Context, id TaskID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.tasks[id]
	if !ok {
		return fmt.Errorf("cancel timer %s: %w", id, ErrTaskNotFound)
	}
	e.stop()
	delete(s.tasks, id)
	return nil
}

// Run — жизненный цикл под rungroup: перенимает базовый ctx (от него срабатывания
// строят таймауты, чтобы SIGTERM обрывал exec'и), блокируется до его отмены,
// затем гасит все таймеры, запрещает новые и дожидается in-flight срабатываний
// (недолго: их ctx уже отменён, потолок — fireTimeout). Возврат nil — штатное
// завершение.
func (s *Scheduler[J]) Run(ctx context.Context) error {
	s.mu.Lock()
	s.baseCtx = ctx
	s.mu.Unlock()

	<-ctx.Done()

	s.mu.Lock()
	s.closed = true
	for _, e := range s.tasks {
		e.stop()
	}
	s.tasks = make(map[TaskID]*entry[J])
	s.mu.Unlock()

	s.firing.Wait()
	return nil
}

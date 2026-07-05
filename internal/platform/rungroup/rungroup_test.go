package rungroup

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWait_EmptyGroup(t *testing.T) {
	g, _ := New(context.Background())
	if err := g.Wait(); err != nil {
		t.Errorf("Wait() = %v, want nil", err)
	}
}

func TestWait_AllNil(t *testing.T) {
	g, _ := New(context.Background())
	g.Go(func() error { return nil })
	g.Go(func() error { return nil })
	if err := g.Wait(); err != nil {
		t.Errorf("Wait() = %v, want nil", err)
	}
}

// Первая ошибка отменяет контекст остальных участников и возвращается из Wait.
func TestWait_FirstErrorCancelsOthers(t *testing.T) {
	boom := errors.New("boom")
	g, ctx := New(context.Background())

	g.Go(func() error { return boom })
	g.Go(func() error {
		// Долгоживущий компонент: блокируется до отмены ctx (как Run адаптеров).
		<-ctx.Done()
		return nil
	})

	done := make(chan error, 1)
	go func() { done <- g.Wait() }()
	select {
	case err := <-done:
		if !errors.Is(err, boom) {
			t.Errorf("Wait() = %v, want %v", err, boom)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Wait завис: ошибка первого не отменила контекст второго")
	}
}

// Отмена родительского контекста доходит до производного (штатный shutdown).
func TestNew_ParentCancelPropagates(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	g, ctx := New(parent)
	g.Go(func() error {
		<-ctx.Done()
		return nil
	})

	cancel()
	done := make(chan error, 1)
	go func() { done <- g.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Wait() = %v, want nil (штатная отмена)", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Wait завис: отмена родителя не дошла до производного ctx")
	}
}

package system

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestClock_Now(t *testing.T) {
	before := time.Now()
	got := NewClock().Now()
	if got.Before(before) {
		t.Errorf("Now() = %v, раньше момента вызова %v", got, before)
	}
}

func TestClock_AfterFunc_Fires(t *testing.T) {
	fired := make(chan struct{})
	NewClock().AfterFunc(time.Millisecond, func() { close(fired) })

	select {
	case <-fired:
	case <-time.After(time.Second):
		t.Fatal("таймер не сработал за секунду")
	}
}

func TestClock_AfterFunc_StopPreventsFire(t *testing.T) {
	var n int32
	stop := NewClock().AfterFunc(50*time.Millisecond, func() { atomic.AddInt32(&n, 1) })

	if !stop() {
		t.Fatal("stop() = false, ждём true для ещё не сработавшего таймера")
	}
	time.Sleep(80 * time.Millisecond)

	if got := atomic.LoadInt32(&n); got != 0 {
		t.Errorf("остановленный таймер сработал %d раз(а)", got)
	}
}

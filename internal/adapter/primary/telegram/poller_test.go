package telegram

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	tele "gopkg.in/telebot.v3"
)

// testLogger — slog в никуда: тестам важен поток управления, не вывод.
func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// newOfflineTeleBot — telebot без сети (Offline: true, паттерн из
// middleware/context_test.go) с API-адресом httptest-сервера. Loopback Go
// не проксирует, поэтому HTTPS_PROXY окружения тестам не мешает.
func newOfflineTeleBot(t *testing.T, url string) *tele.Bot {
	t.Helper()
	b, err := tele.NewBot(tele.Settings{Token: "test", URL: url, Offline: true})
	if err != nil {
		t.Fatalf("tele.NewBot: %v", err)
	}
	return b
}

// startPoll запускает Poll в горутине и возвращает канал завершения.
func startPoll(p *resilientPoller, b *tele.Bot, dest chan tele.Update, stop chan struct{}) chan struct{} {
	done := make(chan struct{})
	go func() {
		p.Poll(b, dest, stop)
		close(done)
	}()
	return done
}

func waitDone(t *testing.T, done chan struct{}, d time.Duration, msg string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatal(msg)
	}
}

func TestPoller_DeliversAndAdvancesOffset(t *testing.T) {
	offsets := make(chan string, 8)
	var served atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var params map[string]string
		_ = json.NewDecoder(r.Body).Decode(&params)
		select { // non-blocking: тест читает только первые запросы
		case offsets <- params["offset"]:
		default:
		}
		if served.Add(1) == 1 {
			fmt.Fprint(w, `{"ok":true,"result":[{"update_id":5}]}`)
			return
		}
		time.Sleep(10 * time.Millisecond) // имитация long-poll, чтобы тест не молотил
		fmt.Fprint(w, `{"ok":true,"result":[]}`)
	}))
	defer srv.Close()

	p := newResilientPoller(testLogger(), new(atomic.Bool))
	dest := make(chan tele.Update, 1)
	stop := make(chan struct{})
	done := startPoll(p, newOfflineTeleBot(t, srv.URL), dest, stop)

	select {
	case u := <-dest:
		if u.ID != 5 {
			t.Errorf("update ID = %d, want 5", u.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("апдейт не доставлен")
	}
	if first := <-offsets; first != "1" {
		t.Errorf("первый offset = %q, want \"1\"", first)
	}
	select {
	case second := <-offsets:
		if second != "6" {
			t.Errorf("offset после update_id=5 = %q, want \"6\"", second)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("второго getUpdates не было")
	}

	close(stop)
	waitDone(t, done, 2*time.Second, "Poll не завершился после close(stop)")
}

func TestPoller_BackoffAndRecovery(t *testing.T) {
	var served atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch n := served.Add(1); {
		case n <= 2:
			fmt.Fprint(w, `{"ok":false,"error_code":400,"description":"boom"}`)
		case n == 3:
			fmt.Fprint(w, `{"ok":true,"result":[{"update_id":1}]}`)
		default:
			time.Sleep(10 * time.Millisecond)
			fmt.Fprint(w, `{"ok":true,"result":[]}`)
		}
	}))
	defer srv.Close()

	p := newResilientPoller(testLogger(), new(atomic.Bool))
	p.backoff = backoff{min: time.Millisecond, max: 4 * time.Millisecond}
	dest := make(chan tele.Update, 1)
	stop := make(chan struct{})
	done := startPoll(p, newOfflineTeleBot(t, srv.URL), dest, stop)

	select {
	case u := <-dest:
		if u.ID != 1 {
			t.Errorf("update ID = %d, want 1", u.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("апдейт после восстановления не доставлен")
	}
	if n := served.Load(); n < 3 {
		t.Errorf("запросов = %d, want ≥3 (2 ошибки + успех)", n)
	}

	close(stop)
	waitDone(t, done, 2*time.Second, "Poll не завершился после close(stop)")
}

// Регрессия: close(stop) обязан прерывать backoff-сон, а не ждать его конца.
func TestPoller_StopInterruptsBackoffSleep(t *testing.T) {
	requested := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case requested <- struct{}{}:
		default:
		}
		fmt.Fprint(w, `{"ok":false,"error_code":400,"description":"boom"}`)
	}))
	defer srv.Close()

	p := newResilientPoller(testLogger(), new(atomic.Bool))
	p.backoff = backoff{min: 10 * time.Second, max: 10 * time.Second} // сон дольше теста
	dest := make(chan tele.Update)
	stop := make(chan struct{})
	done := startPoll(p, newOfflineTeleBot(t, srv.URL), dest, stop)

	<-requested // поллер получил ошибку и ушёл в сон
	close(stop)
	waitDone(t, done, time.Second, "Poll спит вместо выхода: select на stop во сне сломан")
}

// Регрессия унаследованного дедлока LongPoller: close(stop) при заблокированной
// отправке в dest (читателя нет) обязан завершать Poll.
func TestPoller_StopUnblocksDestSend(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"ok":true,"result":[{"update_id":1}]}`)
	}))
	defer srv.Close()

	p := newResilientPoller(testLogger(), new(atomic.Bool))
	dest := make(chan tele.Update) // небуферизованный, читателя нет
	stop := make(chan struct{})
	done := startPoll(p, newOfflineTeleBot(t, srv.URL), dest, stop)

	time.Sleep(50 * time.Millisecond) // дать поллеру заблокироваться на dest <- u
	close(stop)
	waitDone(t, done, time.Second, "Poll завис на отправке в dest: дедлок LongPoller не вылечен")
}

// Поллер переключает состояние Telegram-канала (Bot.Connected) ровно в
// state-transition точках: false при уходе в оффлайн, true при восстановлении.
// Сервер держит фазу ошибок, пока тест не увидит false — оба ожидания идут по
// УСТОЙЧИВЫМ состояниям, короткое окно ловить не нужно (иначе тест флакует,
// если горутину теста заспали дольше окна).
func TestPoller_ConnectedTransitions(t *testing.T) {
	var recovered atomic.Bool // false: сервер отвечает ошибками; true: успехом
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !recovered.Load() {
			fmt.Fprint(w, `{"ok":false,"error_code":400,"description":"boom"}`)
			return
		}
		time.Sleep(10 * time.Millisecond)
		fmt.Fprint(w, `{"ok":true,"result":[]}`)
	}))
	defer srv.Close()

	connected := new(atomic.Bool)
	connected.Store(true) // как после connect-фазы Run
	p := newResilientPoller(testLogger(), connected)
	p.backoff = backoff{min: time.Millisecond, max: 4 * time.Millisecond}
	dest := make(chan tele.Update, 1)
	stop := make(chan struct{})
	done := startPoll(p, newOfflineTeleBot(t, srv.URL), dest, stop)

	waitFor := func(want bool, msg string) {
		t.Helper()
		deadline := time.After(2 * time.Second)
		for connected.Load() != want {
			select {
			case <-deadline:
				t.Fatal(msg)
			case <-time.After(2 * time.Millisecond):
			}
		}
	}
	waitFor(false, "оффлайн не переключил connected в false")
	recovered.Store(true)
	waitFor(true, "восстановление не вернуло connected в true")

	close(stop)
	waitDone(t, done, 2*time.Second, "Poll не завершился после close(stop)")
}

// 429: поллер уважает retry_after Telegram вместо своего backoff, и НЕ считает
// флуд оффлайном — канал жив, /health не должен показывать "connecting".
func TestPoller_FloodRespectsRetryAfter(t *testing.T) {
	var served atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if served.Add(1) == 1 {
			fmt.Fprint(w, `{"ok":false,"error_code":429,"description":"Too Many Requests: retry after 1","parameters":{"retry_after":1}}`)
			return
		}
		time.Sleep(10 * time.Millisecond)
		fmt.Fprint(w, `{"ok":true,"result":[]}`)
	}))
	defer srv.Close()

	connected := new(atomic.Bool)
	connected.Store(true)
	p := newResilientPoller(testLogger(), connected)
	p.backoff = backoff{min: time.Millisecond, max: 2 * time.Millisecond} // без flood ретраил бы мгновенно
	dest := make(chan tele.Update, 1)
	stop := make(chan struct{})
	done := startPoll(p, newOfflineTeleBot(t, srv.URL), dest, stop)

	time.Sleep(300 * time.Millisecond)
	if n := served.Load(); n != 1 {
		t.Errorf("запросов за 300ms = %d, want 1 (retry_after=1s игнорируется?)", n)
	}
	if !connected.Load() {
		t.Error("flood переключил connected в false — 429 не оффлайн")
	}

	close(stop)
	waitDone(t, done, 2*time.Second, "Poll не завершился после close(stop)")
}

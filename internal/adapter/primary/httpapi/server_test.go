package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// testLogger — slog в никуда: тестам важен поток управления, не вывод.
func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func testDeps() Deps {
	return Deps{
		TelegramUp: func() bool { return true },
		Logger:     testLogger(),
	}
}

// do прогоняет запрос через полный Handler сервера (роуты + access-лог).
func do(s *Server, method, target string, body io.Reader) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	s.srv.Handler.ServeHTTP(rec, httptest.NewRequest(method, target, body))
	return rec
}

func TestServe_ServesAndShutsDownGracefully(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	s := NewServer(ln.Addr().String(), testDeps())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.serve(ctx, ln) }()

	resp, err := http.Get("http://" + ln.Addr().String() + healthPath)
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	var health healthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Errorf("тело /health не декодируется: %v", err)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("serve() = %v, want nil (штатная отмена)", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serve не завершился после отмены ctx")
	}
}

// Занятый порт — класс config-ошибок: Run обязан вернуть ошибку сразу.
func TestRun_ListenError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	s := NewServer(ln.Addr().String(), testDeps())
	if err := s.Run(context.Background()); err == nil {
		t.Fatal("Run() = nil на занятом порту, want error")
	}
}

func TestHealth_TelegramState(t *testing.T) {
	for _, tc := range []struct {
		name string
		up   bool
		want string
	}{
		{"connected", true, "connected"},
		{"connecting", false, "connecting"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps := testDeps()
			deps.TelegramUp = func() bool { return tc.up }
			rec := do(NewServer("127.0.0.1:0", deps), http.MethodGet, healthPath, nil)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			var health healthResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &health); err != nil {
				t.Fatalf("тело не декодируется: %v", err)
			}
			if health.Status != "ok" || health.Telegram != tc.want {
				t.Errorf("health = %+v, want status=ok telegram=%s", health, tc.want)
			}
		})
	}
}

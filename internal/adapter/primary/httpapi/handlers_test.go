package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/tumour/openwrt-bot/internal/app/device"
	"github.com/tumour/openwrt-bot/internal/domain"
)

// Стабы портов (паттерн репо: стабится порт, use case поверх — реальный).

type stubDhcp struct {
	leases []domain.Device
	err    error
}

func (s *stubDhcp) ListLeases(context.Context) ([]domain.Device, error) { return s.leases, s.err }

type stubMACSet struct {
	macs    []domain.MAC
	added   []domain.MAC
	removed []domain.MAC
	err     error
}

func (s *stubMACSet) Add(_ context.Context, mac domain.MAC) error {
	s.added = append(s.added, mac)
	return s.err
}

func (s *stubMACSet) Remove(_ context.Context, mac domain.MAC) error {
	s.removed = append(s.removed, mac)
	return s.err
}

func (s *stubMACSet) List(context.Context) ([]domain.MAC, error) { return s.macs, s.err }

type stubRateLimits struct {
	limits  map[domain.MAC]domain.Rate
	set     []domain.MAC
	removed []domain.MAC
	err     error
}

func (s *stubRateLimits) Set(_ context.Context, mac domain.MAC, _ domain.Rate) error {
	s.set = append(s.set, mac)
	return s.err
}

func (s *stubRateLimits) Remove(_ context.Context, mac domain.MAC) error {
	s.removed = append(s.removed, mac)
	return s.err
}

func (s *stubRateLimits) List(context.Context) (map[domain.MAC]domain.Rate, error) {
	return s.limits, s.err
}

func mustMAC(t *testing.T, raw string) domain.MAC {
	t.Helper()
	mac, err := domain.NewMAC(raw)
	if err != nil {
		t.Fatalf("NewMAC(%q): %v", raw, err)
	}
	return mac
}

func mustRate(t *testing.T, kbps int) domain.Rate {
	t.Helper()
	rate, err := domain.NewRate(kbps)
	if err != nil {
		t.Fatalf("NewRate(%d): %v", kbps, err)
	}
	return rate
}

func TestDevices_FullMapping(t *testing.T) {
	mac := mustMAC(t, "aa:bb:cc:11:22:33")
	dhcp := &stubDhcp{leases: []domain.Device{
		{MAC: mac, Hostname: "laptop", IP: net.ParseIP("192.168.1.42")},
	}}
	deps := testDeps()
	deps.List = device.NewList(dhcp,
		&stubMACSet{macs: []domain.MAC{mac}}, // banned
		&stubMACSet{macs: []domain.MAC{mac}}, // direct
		&stubRateLimits{limits: map[domain.MAC]domain.Rate{mac: mustRate(t, 512)}},
	)

	rec := do(NewServer("127.0.0.1:0", testLogger(), deps), http.MethodGet, "/api/v1/devices", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var resp devicesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("тело не декодируется: %v", err)
	}
	want := deviceJSON{
		MAC: "aa:bb:cc:11:22:33", Hostname: "laptop", IP: "192.168.1.42",
		Banned: true, Direct: true, LimitKBytesPerSec: 512,
	}
	if len(resp.Devices) != 1 || resp.Devices[0] != want {
		t.Errorf("devices = %+v, want [%+v]", resp.Devices, want)
	}
}

// String() у nil И у пустого (len 0, но не nil) net.IP возвращает "<nil>" —
// в API в обоих случаях обязана уходить пустая строка.
func TestDevices_EmptyIPIsEmptyString(t *testing.T) {
	dhcp := &stubDhcp{leases: []domain.Device{
		{MAC: mustMAC(t, "aa:bb:cc:11:22:33"), Hostname: "ghost-nil"},                 // IP nil
		{MAC: mustMAC(t, "11:22:33:44:55:66"), Hostname: "ghost-empty", IP: net.IP{}}, // пустой, не nil
	}}
	deps := testDeps()
	deps.List = device.NewList(dhcp, &stubMACSet{}, &stubMACSet{}, &stubRateLimits{})

	rec := do(NewServer("127.0.0.1:0", testLogger(), deps), http.MethodGet, "/api/v1/devices", nil)
	var resp devicesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("тело не декодируется: %v", err)
	}
	for _, d := range resp.Devices {
		if d.IP != "" {
			t.Errorf(`%s: ip = %q, want ""`, d.Hostname, d.IP)
		}
	}
}

// Пустой LAN — JSON-массив [], а не null: клиенту не нужен nil-check.
func TestDevices_EmptyIsArrayNotNull(t *testing.T) {
	deps := testDeps()
	deps.List = device.NewList(&stubDhcp{}, &stubMACSet{}, &stubMACSet{}, &stubRateLimits{})

	rec := do(NewServer("127.0.0.1:0", testLogger(), deps), http.MethodGet, "/api/v1/devices", nil)
	if body := rec.Body.String(); !strings.Contains(body, `"devices":[]`) {
		t.Errorf(`тело не содержит "devices":[] — пустой список утёк как null: %s`, body)
	}
}

// mutationFixture — сервер со всеми мутирующими use cases над стабами;
// стабы возвращаются для проверки «какой порт дёрнули».
type mutationFixture struct {
	banned *stubMACSet
	direct *stubMACSet
	limits *stubRateLimits
	server *Server
}

func newMutationFixture(t *testing.T) *mutationFixture {
	t.Helper()
	f := &mutationFixture{banned: &stubMACSet{}, direct: &stubMACSet{}, limits: &stubRateLimits{}}
	deps := testDeps()
	deps.Ban = device.NewBan(f.banned)
	deps.Unban = device.NewUnban(f.banned)
	deps.VPNOff = device.NewDisableVPN(f.direct)
	deps.VPNOn = device.NewEnableVPN(f.direct)
	deps.SetLimit = device.NewSetLimit(f.limits)
	deps.RemoveLimit = device.NewRemoveLimit(f.limits)
	f.server = NewServer("127.0.0.1:0", testLogger(), deps)
	return f
}

// Каждый маршрут дёргает СВОЙ порт — фиксирует, что маршруты не перепутаны.
// MAC в path — с дефисами и в верхнем регистре: NewMAC обязан нормализовать.
func TestMutations_RouteToOwnPort(t *testing.T) {
	const macPath = "AA-BB-CC-11-22-33"
	want := domain.MAC("aa:bb:cc:11:22:33")

	tests := []struct {
		action string
		body   string
		called func(f *mutationFixture) []domain.MAC
	}{
		{"ban", "", func(f *mutationFixture) []domain.MAC { return f.banned.added }},
		{"unban", "", func(f *mutationFixture) []domain.MAC { return f.banned.removed }},
		{"vpnoff", "", func(f *mutationFixture) []domain.MAC { return f.direct.added }},
		{"vpnon", "", func(f *mutationFixture) []domain.MAC { return f.direct.removed }},
		{"limit", `{"kbytes_per_sec":512}`, func(f *mutationFixture) []domain.MAC { return f.limits.set }},
		{"unlimit", "", func(f *mutationFixture) []domain.MAC { return f.limits.removed }},
	}
	for _, tc := range tests {
		t.Run(tc.action, func(t *testing.T) {
			f := newMutationFixture(t)
			rec := do(f.server, http.MethodPost, "/api/v1/devices/"+macPath+"/"+tc.action, strings.NewReader(tc.body))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
			}
			var ok okResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &ok); err != nil || !ok.OK {
				t.Errorf(`тело = %s, want {"ok":true}`, rec.Body)
			}
			if got := tc.called(f); len(got) != 1 || got[0] != want {
				t.Errorf("порт вызван с %v, want [%s] (нормализованный)", got, want)
			}
		})
	}
}

func TestMutations_InvalidMAC(t *testing.T) {
	f := newMutationFixture(t)
	rec := do(f.server, http.MethodPost, "/api/v1/devices/not-a-mac/ban", nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if len(f.banned.added) != 0 {
		t.Error("порт вызван при невалидном MAC")
	}
}

func TestSetLimit_BadRequests(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{"нулевой лимит", `{"kbytes_per_sec":0}`, http.StatusBadRequest},
		{"лимит сверх диапазона", `{"kbytes_per_sec":1000001}`, http.StatusBadRequest},
		{"битый JSON", `{"kbytes`, http.StatusBadRequest},
		{"неизвестное поле", `{"kbps":512}`, http.StatusBadRequest},
		{"хвост после объекта", `{"kbytes_per_sec":512}{"kbytes_per_sec":9}`, http.StatusBadRequest},
		{"мусор после объекта", `{"kbytes_per_sec":512}garbage`, http.StatusBadRequest},
		{"тело больше килобайта", `{"kbytes_per_sec":` + strings.Repeat("1", 2048) + `}`, http.StatusRequestEntityTooLarge},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newMutationFixture(t)
			rec := do(f.server, http.MethodPost, "/api/v1/devices/aa:bb:cc:11:22:33/limit", strings.NewReader(tc.body))

			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, tc.want, rec.Body)
			}
			if len(f.limits.set) != 0 {
				t.Error("порт вызван при невалидном запросе")
			}
		})
	}
}

// Error boundary мутаций — тот же контракт, что у /devices.
func TestMutations_ErrorBoundary(t *testing.T) {
	var logBuf bytes.Buffer
	deps := testDeps()
	deps.Ban = device.NewBan(&stubMACSet{err: errors.New("nft: exit status 1 (stderr: table missing)")})
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	rec := do(NewServer("127.0.0.1:0", logger, deps), http.MethodPost, "/api/v1/devices/aa:bb:cc:11:22:33/ban", nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if body := rec.Body.String(); strings.Contains(body, "stderr") || strings.Contains(body, "exit status") {
		t.Errorf("внутренности ошибки утекли клиенту: %s", body)
	}
	if !strings.Contains(logBuf.String(), "table missing") {
		t.Error("полная ошибка не попала в лог")
	}
}

// Контракт stdlib-mux для машинного клиента: неверный метод — 405 с Allow,
// неизвестный путь — 404 (тела plain-text, осознанно — см. package doc).
func TestMux_MethodAndPathContract(t *testing.T) {
	f := newMutationFixture(t)

	if rec := do(f.server, http.MethodGet, "/api/v1/devices/aa:bb:cc:11:22:33/ban", nil); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET на POST-маршрут: status = %d, want 405", rec.Code)
	}
	if rec := do(f.server, http.MethodPost, "/api/v1/unknown", nil); rec.Code != http.StatusNotFound {
		t.Errorf("неизвестный путь: status = %d, want 404", rec.Code)
	}
}

// Error boundary: клиенту generic-тело без внутренностей exec, полная ошибка — в slog.
func TestDevices_ErrorBoundary(t *testing.T) {
	cause := errors.New("exec dhcp: exit status 1 (stderr: leases corrupted)")
	deps := testDeps()
	deps.List = device.NewList(&stubDhcp{err: cause}, &stubMACSet{}, &stubMACSet{}, &stubRateLimits{})
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	rec := do(NewServer("127.0.0.1:0", logger, deps), http.MethodGet, "/api/v1/devices", nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if body := rec.Body.String(); strings.Contains(body, "stderr") || strings.Contains(body, "exit status") {
		t.Errorf("внутренности ошибки утекли клиенту: %s", body)
	}
	if !strings.Contains(logBuf.String(), "leases corrupted") {
		t.Error("полная ошибка не попала в лог")
	}
}

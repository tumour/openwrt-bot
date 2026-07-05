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

	rec := do(NewServer("127.0.0.1:0", deps), http.MethodGet, "/api/v1/devices", nil)
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

// net.IP(nil).String() возвращает "<nil>" — в API обязана уходить пустая строка.
func TestDevices_NilIPIsEmptyString(t *testing.T) {
	dhcp := &stubDhcp{leases: []domain.Device{{MAC: mustMAC(t, "aa:bb:cc:11:22:33"), Hostname: "ghost"}}}
	deps := testDeps()
	deps.List = device.NewList(dhcp, &stubMACSet{}, &stubMACSet{}, &stubRateLimits{})

	rec := do(NewServer("127.0.0.1:0", deps), http.MethodGet, "/api/v1/devices", nil)
	var resp devicesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("тело не декодируется: %v", err)
	}
	if got := resp.Devices[0].IP; got != "" {
		t.Errorf(`ip = %q, want "" (nil IP)`, got)
	}
}

// Пустой LAN — JSON-массив [], а не null: клиенту не нужен nil-check.
func TestDevices_EmptyIsArrayNotNull(t *testing.T) {
	deps := testDeps()
	deps.List = device.NewList(&stubDhcp{}, &stubMACSet{}, &stubMACSet{}, &stubRateLimits{})

	rec := do(NewServer("127.0.0.1:0", deps), http.MethodGet, "/api/v1/devices", nil)
	if body := rec.Body.String(); !strings.Contains(body, `"devices":[]`) {
		t.Errorf(`тело не содержит "devices":[] — пустой список утёк как null: %s`, body)
	}
}

// Error boundary: клиенту generic-тело без внутренностей exec, полная ошибка — в slog.
func TestDevices_ErrorBoundary(t *testing.T) {
	cause := errors.New("exec dhcp: exit status 1 (stderr: leases corrupted)")
	deps := testDeps()
	deps.List = device.NewList(&stubDhcp{err: cause}, &stubMACSet{}, &stubMACSet{}, &stubRateLimits{})
	var logBuf bytes.Buffer
	deps.Logger = slog.New(slog.NewTextHandler(&logBuf, nil))

	rec := do(NewServer("127.0.0.1:0", deps), http.MethodGet, "/api/v1/devices", nil)
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

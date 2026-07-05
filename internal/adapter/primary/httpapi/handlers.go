package httpapi

import (
	"context"
	"net"
	"net/http"

	"github.com/tumour/openwrt-bot/internal/app/device"
)

// deviceJSON — DTO этого адаптера: JSON-контракт LuCI-панели фиксируется
// здесь, доменная форма наружу не течёт (рефакторинг domain/app не ломает
// панель незаметно).
type deviceJSON struct {
	MAC      string `json:"mac"`
	Hostname string `json:"hostname"`
	IP       string `json:"ip"` // "" — IP неизвестен
	Banned   bool   `json:"banned"`
	Direct   bool   `json:"direct"` // мимо VPN
	// КБайт/с — единица domain.Rate и nftables («kbytes/second»); 0 = лимита
	// нет. Сознательно не «kbps»: то читается как килобиты/с — источник ×8-багов.
	LimitKBytesPerSec int `json:"limit_kbytes_per_sec"`
}

type devicesResponse struct {
	Devices []deviceJSON `json:"devices"`
}

func (s *Server) handleDevices(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	out, err := s.deps.List.Execute(ctx, device.ListInput{})
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	// Ёмкость с нуля: пустой список обязан сериализоваться как [], а не null.
	devices := make([]deviceJSON, 0, len(out.Devices))
	for _, v := range out.Devices {
		devices = append(devices, deviceJSON{
			MAC:               v.Device.MAC.String(),
			Hostname:          v.Device.Hostname,
			IP:                ipString(v.Device.IP),
			Banned:            v.Banned,
			Direct:            v.Direct,
			LimitKBytesPerSec: v.Limit.KBps(),
		})
	}
	s.writeJSON(w, http.StatusOK, devicesResponse{Devices: devices})
}

// ipString — nil-безопасная строка: net.IP(nil).String() возвращает "<nil>",
// в API это мусор — отдаём пустую строку.
func ipString(ip net.IP) string {
	if ip == nil {
		return ""
	}
	return ip.String()
}

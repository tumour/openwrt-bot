package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/tumour/openwrt-bot/internal/app/device"
	"github.com/tumour/openwrt-bot/internal/domain"
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

// ipString — строка без мусора: String() у nil И у пустого (len 0, но не nil)
// net.IP возвращает "<nil>" — проверяем длину, как сам stdlib.
func ipString(ip net.IP) string {
	if len(ip) == 0 {
		return ""
	}
	return ip.String()
}

// macAction — каркас мутаций по MAC: PathValue → NewMAC (валидация на границе,
// как в telegram-handlers: use case по построению получает валидный value
// object, любые ошибки Execute — инфраструктурные → 500) → exec → 200 ok.
// Разница между endpoint'ами — только Input-тип use case, сильнее не обобщить.
func (s *Server) macAction(exec func(ctx context.Context, mac domain.MAC) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw := r.PathValue("mac") // сегмент уже URL-decoded; NewMAC нормализует регистр и '-'
		mac, err := domain.NewMAC(raw)
		if err != nil {
			s.writeError(w, http.StatusBadRequest, "невалидный MAC: "+raw)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
		defer cancel()

		if err := exec(ctx, mac); err != nil {
			s.internalError(w, r, err)
			return
		}
		s.writeJSON(w, http.StatusOK, okResponse{OK: true})
	}
}

// maxLimitBodyBytes — потолок тела /limit; единственное тело в API —
// {"kbytes_per_sec":N}, килобайта хватает с запасом.
const maxLimitBodyBytes = 1 << 10

func (s *Server) handleSetLimit(w http.ResponseWriter, r *http.Request) {
	raw := r.PathValue("mac")
	mac, err := domain.NewMAC(raw)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "невалидный MAC: "+raw)
		return
	}

	var req struct {
		KBytesPerSec int `json:"kbytes_per_sec"`
	}
	// MaxBytesReader получает РАЗВЁРНУТЫЙ writer: маркер close-after-reply при
	// превышении лимита stdlib ставит через приватный интерфейс конкретного
	// типа сервера, и обёртка access-лога его потеряла бы.
	dec := json.NewDecoder(http.MaxBytesReader(baseWriter(w), r.Body, maxLimitBodyBytes))
	// Опечатка в поле у клиента должна всплыть 400-й, а не молчаливым нулём.
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			s.writeError(w, http.StatusRequestEntityTooLarge, "тело запроса слишком большое")
			return
		}
		s.writeError(w, http.StatusBadRequest, "некорректный JSON: "+err.Error())
		return
	}
	// Decoder останавливается на первом JSON-значении — хвост (второй объект,
	// мусор) без этой проверки прошёл бы молча, в обход строгости выше.
	if err := dec.Decode(new(struct{})); !errors.Is(err, io.EOF) {
		s.writeError(w, http.StatusBadRequest, "лишние данные после JSON-объекта")
		return
	}
	rate, err := domain.NewRate(req.KBytesPerSec)
	if err != nil {
		s.writeError(w, http.StatusBadRequest,
			fmt.Sprintf("невалидный лимит (1..1000000 КБ/с): %d", req.KBytesPerSec))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	if _, err := s.deps.SetLimit.Execute(ctx, device.SetLimitInput{MAC: mac, Rate: rate}); err != nil {
		s.internalError(w, r, err)
		return
	}
	s.writeJSON(w, http.StatusOK, okResponse{OK: true})
}

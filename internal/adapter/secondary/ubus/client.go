package ubus

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/tumour/openwrt-bot/internal/adapter/secondary/system"
	"github.com/tumour/openwrt-bot/internal/app/status"
)

// Client реализует status.SystemPort через ubus — родную RPC-систему OpenWrt.
// Команда `ubus call system info` отдаёт JSON-объект с uptime/load/memory.
//
// Обрати внимание: Client НЕ имплементит status.SystemPort явно (Go-стиль) — он
// просто реализует его методы. Когда NewGetStatus(client) вызовут с этим Client'ом,
// Go-компилятор проверит соответствие интерфейсу.
type Client struct {
	runner system.Runner
}

func NewClient(runner system.Runner) *Client {
	return &Client{runner: runner}
}

// ubusSystemInfo — приватный DTO, отражает JSON-вывод `ubus call system info`.
// Делаем отдельный тип, а не парсим в status.Snapshot напрямую, чтобы изолировать
// формат ubus от уровня app. Если завтра ubus поменяет схему — поправим только тут.
type ubusSystemInfo struct {
	Uptime int64     `json:"uptime"` // секунды
	Load   [3]uint64 `json:"load"`   // x65536, см. https://openwrt.org/docs/techref/ubus
	Memory struct {
		Total uint64 `json:"total"` // байты
		Free  uint64 `json:"free"`
	} `json:"memory"`
}

// Snapshot вызывает ubus и преобразует ответ в DTO use-case-уровня (status.Snapshot).
func (c *Client) Snapshot(ctx context.Context) (status.Snapshot, error) {
	out, err := c.runner.Run(ctx, "ubus", "call", "system", "info")
	if err != nil {
		return status.Snapshot{}, fmt.Errorf("ubus call system info: %w", err)
	}

	var info ubusSystemInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return status.Snapshot{}, fmt.Errorf("parse ubus output: %w", err)
	}

	return status.Snapshot{
		Uptime:     time.Duration(info.Uptime) * time.Second,
		MemTotalKB: info.Memory.Total / 1024,
		MemFreeKB:  info.Memory.Free / 1024,
		LoadAvg1:   float64(info.Load[0]) / 65536.0,
		LoadAvg5:   float64(info.Load[1]) / 65536.0,
		LoadAvg15:  float64(info.Load[2]) / 65536.0,
		// TempCelsius здесь не заполняем — это зона ответственности ThermalPort
		// (adapter/secondary/thermal), GetStatus подмешивает её к этому снапшоту.
	}, nil
}

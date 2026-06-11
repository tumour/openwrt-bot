// Package librespeed — реализация network.SpeedTestPort через librespeed-cli
// (apk-пакет на OpenWrt). Вызывает `librespeed-cli --json` и парсит результат.
package librespeed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"

	"github.com/tumour/openwrt-bot/internal/adapter/secondary/system"
	"github.com/tumour/openwrt-bot/internal/app/network"
)

// Client реализует network.SpeedTestPort. serverID опционален: пусто → librespeed
// сам выбирает сервер (нередко далёкий → заниженные цифры), непустой → пиннуем
// конкретный сервер из списка librespeed (`librespeed-cli --list`).
type Client struct {
	runner   system.Runner
	serverID string
}

func NewClient(runner system.Runner, serverID string) *Client {
	return &Client{runner: runner, serverID: serverID}
}

// librespeedResult — приватный DTO под JSON-вывод `librespeed-cli --json`.
// Вывод — массив из одного объекта. download/upload в Mbps, ping/jitter в ms.
type librespeedResult struct {
	Server struct {
		Name string `json:"name"`
	} `json:"server"`
	Ping     float64 `json:"ping"`
	Jitter   float64 `json:"jitter"`
	Upload   float64 `json:"upload"`
	Download float64 `json:"download"`
}

// Measure запускает замер и преобразует JSON в network.SpeedResult.
func (c *Client) Measure(ctx context.Context) (network.SpeedResult, error) {
	args := []string{"--json"}
	if c.serverID != "" {
		args = append(args, "--server", c.serverID)
	}

	out, err := c.runner.Run(ctx, "librespeed-cli", args...)
	if err != nil {
		// Бинаря нет на роутере — переводим в типизированную ошибку app-уровня,
		// текст подсказки для юзера — забота primary adapter'а (handler).
		if errors.Is(err, exec.ErrNotFound) {
			return network.SpeedResult{}, fmt.Errorf("librespeed-cli: %w", network.ErrToolMissing)
		}
		return network.SpeedResult{}, fmt.Errorf("run librespeed-cli: %w", err)
	}

	var results []librespeedResult
	if err := json.Unmarshal(out, &results); err != nil {
		return network.SpeedResult{}, fmt.Errorf("parse librespeed output: %w", err)
	}
	if len(results) == 0 {
		return network.SpeedResult{}, fmt.Errorf("librespeed-cli returned empty result")
	}

	r := results[0]
	return network.SpeedResult{
		DownloadMbps: r.Download,
		UploadMbps:   r.Upload,
		PingMs:       r.Ping,
		JitterMs:     r.Jitter,
		Server:       r.Server.Name,
	}, nil
}

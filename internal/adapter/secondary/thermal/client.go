// Package thermal — реализация status.ThermalPort. Читает температуру CPU из
// sysfs-файла термозоны. На OpenWrt путь по умолчанию —
// /sys/class/thermal/thermal_zone0/temp, значение в милли-градусах Цельсия
// (целое, напр. "55300" → 55.3 °C).
package thermal

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/tumour/openwrt-bot/internal/adapter/secondary/system"
)

// Client реализует status.ThermalPort. Путь к термозоне вынесен в конструктор:
// разные платформы кладут датчик CPU в разные зоны (thermal_zone0/1/...), плюс
// это упрощает тесты. Read идёт через тот же system.FileReader, что и dhcp-клиент.
type Client struct {
	reader system.FileReader
	path   string
}

func NewClient(reader system.FileReader, path string) *Client {
	return &Client{reader: reader, path: path}
}

// Temperature читает файл термозоны и возвращает температуру в °C.
func (c *Client) Temperature(ctx context.Context) (float64, error) {
	data, err := c.reader.ReadFile(ctx, c.path)
	if err != nil {
		return 0, fmt.Errorf("read thermal zone: %w", err)
	}
	return parseMilliCelsius(data)
}

// parseMilliCelsius — чистая функция: "55300\n" → 55.3. sysfs thermal хранит
// температуру в тысячных долях градуса Цельсия (millidegree Celsius).
func parseMilliCelsius(data []byte) (float64, error) {
	s := strings.TrimSpace(string(data))
	milli, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse thermal value %q: %w", s, err)
	}
	return float64(milli) / 1000.0, nil
}

// Package dhcp — реализация device.DhcpPort. Читает файл лиз dnsmasq
// (по умолчанию /tmp/dhcp.leases на OpenWrt) и парсит в domain.Device.
//
// Формат строки (5 полей, разделитель пробел):
//   <expiry> <mac> <ip> <hostname|*> <client-id|*>
// Пример:
//   1700000000 aa:bb:cc:dd:ee:ff 192.168.88.42 my-laptop 01:aa:bb:cc:dd:ee:ff
package dhcp

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/tumour/openwrt-bot/internal/adapter/secondary/system"
	"github.com/tumour/openwrt-bot/internal/domain"
)

// Client — реализация device.DhcpPort. LeasesPath вынесен в конструктор для
// тестируемости и для возможности подменить путь (например на /var/lib/...).
type Client struct {
	reader     system.FileReader
	leasesPath string
}

func NewClient(reader system.FileReader, leasesPath string) *Client {
	return &Client{reader: reader, leasesPath: leasesPath}
}

// ListLeases читает leases-файл и возвращает список устройств.
// Возвращает пустой slice (не nil), если файла нет или он пуст — это валидное
// состояние (никого не зарегистрировано), а не ошибка для вызывающего.
func (c *Client) ListLeases(ctx context.Context) ([]domain.Device, error) {
	data, err := c.reader.ReadFile(ctx, c.leasesPath)
	if err != nil {
		return nil, fmt.Errorf("read leases: %w", err)
	}
	return parseLeases(data), nil
}

// parseLeases — чистая функция без зависимостей, легко тестируется отдельно.
// Невалидные строки (битые формат / MAC) молча скипаются — leases-файл может
// быть в момент чтения частично записан или содержать "*"-плейсхолдеры.
func parseLeases(data []byte) []domain.Device {
	devices := make([]domain.Device, 0)
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 4 {
			continue
		}
		mac, err := domain.NewMAC(fields[1])
		if err != nil {
			continue
		}
		hostname := fields[3]
		if hostname == "*" {
			hostname = ""
		}
		devices = append(devices, domain.Device{
			MAC:      mac,
			Hostname: hostname,
			IP:       net.ParseIP(fields[2]),
		})
	}
	return devices
}

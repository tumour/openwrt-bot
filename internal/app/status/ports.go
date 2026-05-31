package status

import (
	"context"
	"time"
)

// SystemPort отдаёт системную информацию о роутере. Реализуется через ubus
// (impl в adapter/secondary/ubus) или чтение /proc (impl в adapter/secondary/system).
type SystemPort interface {
	Snapshot(ctx context.Context) (Snapshot, error)
}

// ThermalPort отдаёт температуру CPU роутера в градусах Цельсия. Отдельный порт,
// а не поле SystemPort, потому что источник другой — sysfs-файл термозоны
// (/sys/class/thermal/...), а не ubus. GetStatus оркестрирует оба порта, как
// device.List оркестрирует DhcpPort + NftPort.
type ThermalPort interface {
	Temperature(ctx context.Context) (float64, error)
}

// Snapshot — мгновенный срез системных метрик. Живёт здесь, а не в domain/, потому
// что "load average" / "температура" — это технические метрики хоста, а не часть
// доменной модели "устройство в сети". Из таких метрик не выводятся доменные правила.
type Snapshot struct {
	Uptime      time.Duration
	MemTotalKB  uint64
	MemFreeKB   uint64
	LoadAvg1    float64
	LoadAvg5    float64
	LoadAvg15   float64
	TempCelsius float64 // 0, если датчика нет
}

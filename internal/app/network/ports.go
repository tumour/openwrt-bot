package network

import "context"

// SpeedTestPort замеряет скорость интернет-канала роутера. Реализуется через
// librespeed-cli (impl в adapter/secondary/librespeed). Замер локальный для
// роутера — трафик идёт напрямую через ISP, не через VPN (TPROXY ловит только
// форвард LAN, не output самого роутера).
type SpeedTestPort interface {
	Measure(ctx context.Context) (SpeedResult, error)
}

// SpeedResult — итог замера. Живёт здесь, а не в domain/, потому что это
// техническая метрика канала, а не часть доменной модели "устройство в сети".
type SpeedResult struct {
	DownloadMbps float64
	UploadMbps   float64
	PingMs       float64
	JitterMs     float64
	Server       string // имя выбранного сервера, "" если неизвестно
}

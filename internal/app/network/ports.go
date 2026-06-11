package network

import (
	"context"
	"errors"
)

// ErrToolMissing — инструмент замера не установлен на роутере. Типизирована,
// чтобы primary adapter мог показать юзеру подсказку по установке, не завязываясь
// на текст ошибки secondary adapter'а.
var ErrToolMissing = errors.New("speedtest tool is not installed")

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

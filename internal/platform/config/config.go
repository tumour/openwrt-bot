package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

// Config — всё конфигурируется через env. Никаких файлов / YAML / Viper.
// caarlos0/env по тегам распарсит ENV → struct, и если required-поле пустое
// или required=true проваливается — вернёт ошибку. Это даёт "fail fast" на старте.
type Config struct {
	BotToken       string  `env:"BOT_TOKEN,required"`
	AllowedChatIDs []int64 `env:"ALLOWED_CHAT_IDS,required" envSeparator:","`
	LogLevel       string  `env:"LOG_LEVEL" envDefault:"info"`
	DhcpLeasesPath string  `env:"DHCP_LEASES_PATH" envDefault:"/tmp/dhcp.leases"`
	// ThermalZonePath — sysfs-файл с температурой CPU для /status. На разных
	// платформах датчик CPU лежит в разных зонах (thermal_zone0/1/...); если на
	// железе зоны нет вовсе — /status просто не покажет строку Temp (см. GetStatus).
	ThermalZonePath string `env:"THERMAL_ZONE_PATH" envDefault:"/sys/class/thermal/thermal_zone0/temp"`
	// SpeedTestServerID — ID сервера librespeed для /speedtest. Пусто → авто-выбор
	// (librespeed нередко берёт далёкий сервер → заниженные цифры). Список ID:
	// `librespeed-cli --list` на роутере.
	SpeedTestServerID string `env:"SPEEDTEST_SERVER_ID" envDefault:""`
}

// Load читает конфиг из ENV. Возвращает ошибку, если обязательные переменные пусты.
func Load() (Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse env: %w", err)
	}
	return cfg, nil
}

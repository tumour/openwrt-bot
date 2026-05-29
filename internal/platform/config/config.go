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
}

// Load читает конфиг из ENV. Возвращает ошибку, если обязательные переменные пусты.
func Load() (Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse env: %w", err)
	}
	return cfg, nil
}

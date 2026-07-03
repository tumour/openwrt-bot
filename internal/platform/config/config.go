package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

// Config — всё конфигурируется через env. Никаких файлов / YAML / Viper.
// caarlos0/env по тегам распарсит ENV → struct, и если required-поле пустое
// или required=true проваливается — вернёт ошибку. Это даёт "fail fast" на старте.
type Config struct {
	BotToken string `env:"BOT_TOKEN,required"`
	// AdminUserID — Telegram user ID первого админа (ровно один). Без него бот
	// не стартует: списком пользователей управляют только носители manage_users,
	// и хотя бы один обязан существовать. Используется ТОЛЬКО как сид при
	// старте (access.Seed): дальше пользователи живут в хранилище, env не
	// источник правды. Остальные получают доступ через approve-flow (/start).
	AdminUserID int64 `env:"ADMIN_USER_ID,required"`
	// DatabaseDir — каталог runtime-данных бота. Внутри по подкаталогу на
	// движок: database/json (текущий), завтра database/sqlite и т.д.
	// На роутере живёт в /etc/openwrt-bot/ — переживает sysupgrade.
	DatabaseDir    string `env:"DATABASE_DIR" envDefault:"/etc/openwrt-bot/database"`
	LogLevel       string `env:"LOG_LEVEL" envDefault:"info"`
	DhcpLeasesPath string `env:"DHCP_LEASES_PATH" envDefault:"/tmp/dhcp.leases"`
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
	if cfg.AdminUserID <= 0 {
		return Config{}, fmt.Errorf("ADMIN_USER_ID=%d: нужен Telegram user ID админа — без него ботом никто не сможет управлять", cfg.AdminUserID)
	}
	return cfg, nil
}

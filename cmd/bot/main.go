// Package main — composition root. Единственное место, где собирается граф
// зависимостей: adapters → use cases → handlers → bot. Никакой бизнес-логики тут нет.
package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/tumour/openwrt-bot/internal/adapter/primary/telegram"
	"github.com/tumour/openwrt-bot/internal/adapter/primary/telegram/handler"
	"github.com/tumour/openwrt-bot/internal/adapter/secondary/accessjson"
	"github.com/tumour/openwrt-bot/internal/adapter/secondary/dhcp"
	"github.com/tumour/openwrt-bot/internal/adapter/secondary/librespeed"
	"github.com/tumour/openwrt-bot/internal/adapter/secondary/nftables"
	"github.com/tumour/openwrt-bot/internal/adapter/secondary/system"
	"github.com/tumour/openwrt-bot/internal/adapter/secondary/thermal"
	"github.com/tumour/openwrt-bot/internal/adapter/secondary/ubus"
	"github.com/tumour/openwrt-bot/internal/app/access"
	"github.com/tumour/openwrt-bot/internal/app/device"
	"github.com/tumour/openwrt-bot/internal/app/network"
	"github.com/tumour/openwrt-bot/internal/app/status"
	"github.com/tumour/openwrt-bot/internal/platform/config"
	"github.com/tumour/openwrt-bot/internal/platform/logger"
	"github.com/tumour/openwrt-bot/internal/platform/shutdown"
)

func main() {
	if err := run(); err != nil {
		// На этом этапе у нас может ещё не быть logger'а (упали в Load).
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

// run возвращает error вместо вызова os.Exit — это паттерн Mat Ryer:
// "main вызывает run, run возвращает ошибку". Это делает main тривиальным
// и позволяет в будущем тестировать run() из integration-теста.
func run() error {
	// 1. Конфиг — fail fast, если что-то не задано в env.
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// 2. Логгер по уровню из конфига.
	log := logger.New(cfg.LogLevel)

	// 3. Контекст, отменяемый по SIGINT/SIGTERM — для graceful shutdown.
	ctx, stop := shutdown.Context()
	defer stop()

	// 4. Secondary adapters (вниз по графу).
	runner := system.NewExecRunner()
	fileReader := system.NewOSFileReader()
	fileWriter := system.NewOSFileWriter()
	ubusClient := ubus.NewClient(runner)
	thermalClient := thermal.NewClient(fileReader, cfg.ThermalZonePath)
	// Два nft-сета на одной таблице: бан-лист и vpn-обход. Правила, смотрящие
	// на сеты (drop / mark 0xff), создаёт bootstrap в init.d, бот управляет
	// только элементами.
	nftBanned := nftables.NewClient(runner, "inet fw4", "banned_macs")
	nftDirect := nftables.NewClient(runner, "inet fw4", "vpn_direct_macs")
	dhcpClient := dhcp.NewClient(fileReader, cfg.DhcpLeasesPath)
	speedClient := librespeed.NewClient(runner, cfg.SpeedTestServerID)
	// Хранилище доступа: JSON-движок в <DatabaseDir>/json (см. accessjson).
	accessStore := accessjson.New(fileReader, fileWriter, filepath.Join(cfg.DatabaseDir, "json"))

	// 5. Use cases (выше). Каждый получает порты через конструктор.
	users, roles := accessStore.Users(), accessStore.Roles()
	getStatusUC := status.NewGetStatus(ubusClient, thermalClient)
	banUC := device.NewBan(nftBanned)
	unbanUC := device.NewUnban(nftBanned)
	listUC := device.NewList(dhcpClient, nftBanned, nftDirect)
	speedTestUC := network.NewRunSpeedTest(speedClient)
	vpnOffUC := device.NewDisableVPN(nftDirect)
	vpnOnUC := device.NewEnableVPN(nftDirect)
	checkUC := access.NewCheck(users, roles)

	// 5b. Bootstrap доступа: каталог стора, встроенные роли (admin с догоном
	// каталога прав), env-админ. Без валидного ADMIN_USER_ID бот не стартует.
	if err := accessStore.Init(ctx); err != nil {
		return fmt.Errorf("access store: %w", err)
	}
	if err := access.NewSeed(users, roles).Execute(ctx, access.SeedInput{AdminID: cfg.AdminUserID}); err != nil {
		return fmt.Errorf("access seed: %w", err)
	}

	// 6. Handlers (Primary). Принимают use cases.
	handlers := telegram.Handlers{
		Status:    handler.NewStatus(getStatusUC),
		Ban:       handler.NewBan(banUC),
		Unban:     handler.NewUnban(unbanUC),
		Devices:   handler.NewDevices(listUC, banUC, unbanUC, vpnOffUC, vpnOnUC),
		SpeedTest: handler.NewSpeedTest(speedTestUC),
		VPNOff:    handler.NewVPNOff(vpnOffUC),
		VPNOn:     handler.NewVPNOn(vpnOnUC),
		Access: handler.NewAccess(
			access.NewRequestAccess(users, roles),
			access.NewApprove(users, roles),
			access.NewReject(users, roles),
			access.NewListUsers(users, roles),
			access.NewListRoles(users, roles),
			access.NewSetRole(users, roles),
			access.NewRemoveUser(users, roles),
		),
	}

	// 7. Bot. Собирается из cfg, logger, чекера доступа и handlers.
	bot, err := telegram.NewBot(telegram.Config{Token: cfg.BotToken}, log, checkUC, handlers)
	if err != nil {
		return err
	}

	log.Info("openwrt-bot started", "admin_user_id", cfg.AdminUserID)
	return bot.Run(ctx)
}

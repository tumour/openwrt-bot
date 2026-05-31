// Package main — composition root. Единственное место, где собирается граф
// зависимостей: adapters → use cases → handlers → bot. Никакой бизнес-логики тут нет.
package main

import (
	"log/slog"
	"os"

	"github.com/tumour/openwrt-bot/internal/adapter/primary/telegram"
	"github.com/tumour/openwrt-bot/internal/adapter/primary/telegram/handler"
	"github.com/tumour/openwrt-bot/internal/adapter/secondary/dhcp"
	"github.com/tumour/openwrt-bot/internal/adapter/secondary/librespeed"
	"github.com/tumour/openwrt-bot/internal/adapter/secondary/nftables"
	"github.com/tumour/openwrt-bot/internal/adapter/secondary/system"
	"github.com/tumour/openwrt-bot/internal/adapter/secondary/thermal"
	"github.com/tumour/openwrt-bot/internal/adapter/secondary/ubus"
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
	ubusClient := ubus.NewClient(runner)
	thermalClient := thermal.NewClient(fileReader, cfg.ThermalZonePath)
	nftClient := nftables.NewClient(runner, "inet fw4", "banned_macs")
	dhcpClient := dhcp.NewClient(fileReader, cfg.DhcpLeasesPath)
	speedClient := librespeed.NewClient(runner, cfg.SpeedTestServerID)

	// 5. Use cases (выше). Каждый получает порты через конструктор.
	getStatusUC := status.NewGetStatus(ubusClient, thermalClient)
	banUC := device.NewBan(nftClient)
	unbanUC := device.NewUnban(nftClient)
	listUC := device.NewList(dhcpClient, nftClient)
	speedTestUC := network.NewRunSpeedTest(speedClient)

	// 6. Handlers (Primary). Принимают use cases.
	handlers := telegram.Handlers{
		Status:    handler.NewStatus(getStatusUC),
		Ban:       handler.NewBan(banUC),
		Unban:     handler.NewUnban(unbanUC),
		List:      handler.NewList(listUC),
		SpeedTest: handler.NewSpeedTest(speedTestUC),
	}

	// 7. Bot. Собирается из cfg, logger, handlers — больше ничего не нужно.
	bot, err := telegram.NewBot(
		telegram.Config{Token: cfg.BotToken, AllowedChatIDs: cfg.AllowedChatIDs},
		log,
		handlers,
	)
	if err != nil {
		return err
	}

	log.Info("openwrt-bot started", "allowed_chats", len(cfg.AllowedChatIDs))
	return bot.Run(ctx)
}

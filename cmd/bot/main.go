// Package main — composition root. Единственное место, где собирается граф
// зависимостей: adapters → use cases → handlers → bot. Никакой бизнес-логики тут нет.
package main

import (
	"log/slog"
	"os"

	"github.com/tumour/openwrt-bot/internal/adapter/primary/httpapi"
	"github.com/tumour/openwrt-bot/internal/adapter/primary/telegram"
	"github.com/tumour/openwrt-bot/internal/adapter/primary/telegram/handler"
	"github.com/tumour/openwrt-bot/internal/adapter/secondary/dhcp"
	"github.com/tumour/openwrt-bot/internal/adapter/secondary/librespeed"
	"github.com/tumour/openwrt-bot/internal/adapter/secondary/nftables"
	"github.com/tumour/openwrt-bot/internal/adapter/secondary/schedule"
	"github.com/tumour/openwrt-bot/internal/adapter/secondary/system"
	"github.com/tumour/openwrt-bot/internal/adapter/secondary/thermal"
	"github.com/tumour/openwrt-bot/internal/adapter/secondary/ubus"
	"github.com/tumour/openwrt-bot/internal/app/device"
	"github.com/tumour/openwrt-bot/internal/app/network"
	"github.com/tumour/openwrt-bot/internal/app/status"
	"github.com/tumour/openwrt-bot/internal/app/timer"
	"github.com/tumour/openwrt-bot/internal/domain"
	"github.com/tumour/openwrt-bot/internal/platform/config"
	"github.com/tumour/openwrt-bot/internal/platform/logger"
	"github.com/tumour/openwrt-bot/internal/platform/rungroup"
	"github.com/tumour/openwrt-bot/internal/platform/scheduler"
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
	// Два nft-сета в собственной таблице бота: бан-лист и vpn-обход. Правила,
	// смотрящие на сеты (drop / mark 0xff), создаёт bootstrap в init.d, бот
	// управляет только элементами. Таблица своя, а не inet fw4, потому что
	// `fw4 reload` делает flush своей таблицы и вычищал бы правила (сеты
	// выживали — бот показывал «забанен», а drop уже не работал).
	nftBanned := nftables.NewClient(runner, "inet openwrt_bot", "banned_macs")
	nftDirect := nftables.NewClient(runner, "inet openwrt_bot", "vpn_direct_macs")
	// Лимиты скорости живут в отдельной netdev-таблице (policing на ingress/egress
	// br-lan); скелет так же создаёт init.d, бот — только limit-объекты и элементы map.
	nftLimits := nftables.NewRateLimiter(runner, "netdev openwrt_bot")
	dhcpClient := dhcp.NewClient(fileReader, cfg.DhcpLeasesPath)
	speedClient := librespeed.NewClient(runner, cfg.SpeedTestServerID)
	sysClock := system.NewClock()

	// 5. Use cases (выше). Каждый получает порты через конструктор.
	getStatusUC := status.NewGetStatus(ubusClient, thermalClient)
	banUC := device.NewBan(nftBanned)
	unbanUC := device.NewUnban(nftBanned)
	listUC := device.NewList(dhcpClient, nftBanned, nftDirect, nftLimits)
	speedTestUC := network.NewRunSpeedTest(speedClient)
	vpnOffUC := device.NewDisableVPN(nftDirect)
	vpnOnUC := device.NewEnableVPN(nftDirect)
	setLimitUC := device.NewSetLimit(nftLimits)
	removeLimitUC := device.NewRemoveLimit(nftLimits)

	// Планировщик отложенных задач. Обобщённый движок (тип задачи — domain.DeviceJob)
	// на системных часах; deviceRunner переводит действие в вызов use case device.*
	// при срабатывании — правило «повтор = no-op» переиспользуется, рассинхрона с
	// кнопками нет. Таймеры живут в памяти процесса (теряются при рестарте).
	// schedule.Adapter подгоняет движок под порт timer.SchedulerPort (app по
	// dependency rule не видит platform).
	deviceTimers := scheduler.New[domain.DeviceJob](sysClock, deviceRunner{
		ban: banUC, unban: unbanUC, vpnOff: vpnOffUC, vpnOn: vpnOnUC, log: log,
	})
	timerPort := schedule.NewAdapter(deviceTimers)
	scheduleTimerUC := timer.NewSchedule(timerPort)
	listTimersUC := timer.NewList(timerPort)
	cancelTimerUC := timer.NewCancel(timerPort)

	// 6. Handlers (Primary). Принимают use cases.
	handlers := telegram.Handlers{
		Status:    handler.NewStatus(getStatusUC),
		Ban:       handler.NewBan(banUC),
		Unban:     handler.NewUnban(unbanUC),
		Devices:   handler.NewDevices(listUC, banUC, unbanUC, vpnOffUC, vpnOnUC, setLimitUC, removeLimitUC),
		SpeedTest: handler.NewSpeedTest(speedTestUC),
		VPNOff:    handler.NewVPNOff(vpnOffUC),
		VPNOn:     handler.NewVPNOn(vpnOnUC),
		Limit:     handler.NewLimit(setLimitUC),
		Unlimit:   handler.NewUnlimit(removeLimitUC),
		Timers:    handler.NewTimers(scheduleTimerUC, listTimersUC, cancelTimerUC, listUC),
	}

	// 7. Bot. Собирается из cfg, logger, handlers — больше ничего не нужно.
	bot, err := telegram.NewBot(
		telegram.Config{Token: cfg.BotToken, AllowedUserIDs: cfg.AllowedUserIDs},
		log,
		handlers,
	)
	if err != nil {
		return err
	}

	log.Info("openwrt-bot started", "allowed_users", len(cfg.AllowedUserIDs), "http_api", cfg.HTTPAddr != "")

	// 8. Общая судьба: ошибка одного компонента гасит остальных, SIGTERM гасит
	// всех. Бот и планировщик крутятся всегда (таймеры — часть бота); HTTP API —
	// только если задан адрес. Раньше при выключенном API был один блокирующий
	// Run, теперь долгоживущих компонентов минимум два, поэтому rungroup всегда.
	g, gctx := rungroup.New(ctx)
	g.Go(func() error { return bot.Run(gctx) })
	g.Go(func() error { return deviceTimers.Run(gctx) })

	if cfg.HTTPAddr != "" {
		// HTTP API — второй primary adapter поверх ТЕХ ЖЕ экземпляров use cases.
		// TelegramUp — единственная связь primary↔primary, и живёт она только здесь.
		api := httpapi.NewServer(cfg.HTTPAddr, log, httpapi.Deps{
			List:        listUC,
			Ban:         banUC,
			Unban:       unbanUC,
			VPNOff:      vpnOffUC,
			VPNOn:       vpnOnUC,
			SetLimit:    setLimitUC,
			RemoveLimit: removeLimitUC,
			TelegramUp:  bot.Connected,
		})
		g.Go(func() error { return api.Run(gctx) })
	}

	return g.Wait()
}

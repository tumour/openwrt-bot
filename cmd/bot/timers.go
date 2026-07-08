package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/tumour/openwrt-bot/internal/app/device"
	"github.com/tumour/openwrt-bot/internal/domain"
)

// deviceRunner выполняет отложенную device-задачу: переводит domain.Action в
// вызов соответствующего use case device.* — единственное место связки таймеров
// с устройствами. Правило «повтор = no-op» живёт в самих use cases и здесь
// переиспользуется, поэтому таймер и кнопки карточки не расходятся. Реализует
// scheduler.Runner[domain.DeviceJob] структурно. Исход срабатывания логирует
// сам (движок его не трогает) — диагноз виден в `bot log`.
type deviceRunner struct {
	ban    *device.Ban
	unban  *device.Unban
	vpnOff *device.DisableVPN
	vpnOn  *device.EnableVPN
	log    *slog.Logger
}

// Run — точка входа планировщика при срабатывании таймера.
func (r deviceRunner) Run(ctx context.Context, job domain.DeviceJob) error {
	if err := r.dispatch(ctx, job); err != nil {
		r.log.Error("таймер: действие не выполнено", "mac", job.MAC, "action", job.Action, "err", err)
		return err
	}
	r.log.Info("таймер сработал", "mac", job.MAC, "action", job.Action)
	return nil
}

func (r deviceRunner) dispatch(ctx context.Context, job domain.DeviceJob) error {
	switch job.Action {
	case domain.ActionBan:
		_, err := r.ban.Execute(ctx, device.BanInput{MAC: job.MAC})
		return err
	case domain.ActionUnban:
		_, err := r.unban.Execute(ctx, device.UnbanInput{MAC: job.MAC})
		return err
	case domain.ActionVPNOff:
		_, err := r.vpnOff.Execute(ctx, device.DisableVPNInput{MAC: job.MAC})
		return err
	case domain.ActionVPNOn:
		_, err := r.vpnOn.Execute(ctx, device.EnableVPNInput{MAC: job.MAC})
		return err
	default:
		return fmt.Errorf("неизвестное действие таймера: %q", job.Action)
	}
}

// Package nftables — реализация device.MACSetPort через утилиту `nft`.
// Каждый Client работает с одним сетом MAC-адресов; правила, которые на сет
// смотрят (drop для banned_macs, mark 0xff для vpn_direct_macs), создаёт
// bootstrap в init.d, не бот.
package nftables

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/tumour/openwrt-bot/internal/adapter/secondary/system"
	"github.com/tumour/openwrt-bot/internal/domain"
)

// Client управляет конкретным nftables-сетом (например `inet openwrt_bot banned_macs`).
// Table и Set вынесены в конструктор, чтобы можно было использовать с разными
// firewall-сетапами (fw4, custom table) без правки кода.
type Client struct {
	runner system.Runner
	table  string // "inet openwrt_bot"
	set    string // "banned_macs"
}

func NewClient(runner system.Runner, table, set string) *Client {
	return &Client{runner: runner, table: table, set: set}
}

// Add добавляет MAC в сет. Если уже есть — возвращает domain.ErrAlreadyInSet
// (распознаётся по тексту stderr). Это позволяет use case'у различать классы ошибок.
func (c *Client) Add(ctx context.Context, mac domain.MAC) error {
	_, err := c.runner.Run(ctx, "nft", "add", "element", c.table, c.set,
		fmt.Sprintf("{ %s }", mac))
	if err != nil {
		if isAlreadyExists(err) {
			return fmt.Errorf("%s: %w", mac, domain.ErrAlreadyInSet)
		}
		return fmt.Errorf("nft add element: %w", err)
	}
	return nil
}

// Remove удаляет MAC из сета. Если не было — domain.ErrNotInSet.
func (c *Client) Remove(ctx context.Context, mac domain.MAC) error {
	_, err := c.runner.Run(ctx, "nft", "delete", "element", c.table, c.set,
		fmt.Sprintf("{ %s }", mac))
	if err != nil {
		if isNotFound(err) {
			return fmt.Errorf("%s: %w", mac, domain.ErrNotInSet)
		}
		return fmt.Errorf("nft delete element: %w", err)
	}
	return nil
}

// List читает сет и возвращает список MAC. Парсит человеко-читаемый вывод
// `nft list set` — JSON-режим `nft -j` мог бы быть удобнее, но не во всех версиях
// nftables на OpenWrt он стабилен, поэтому держим текстовый.
func (c *Client) List(ctx context.Context) ([]domain.MAC, error) {
	out, err := c.runner.Run(ctx, "nft", "list", "set", c.table, c.set)
	if err != nil {
		return nil, fmt.Errorf("nft list set: %w", err)
	}
	return parseSetElements(out), nil
}

// nft пишет причину отказа в stderr (netlink strerror). Матчим ТОЛЬКО stderr
// типизированной system.ExecError: в Error() входят аргументы команды, и поиск
// подстроки по всей строке давал бы ложные срабатывания. Ключевые слова, а не
// полный текст — сообщения слегка гуляют между версиями nftables; локаль
// зафиксирована (LC_ALL=C в ExecRunner).
func stderrContains(err error, keywords ...string) bool {
	var ee *system.ExecError
	if !errors.As(err, &ee) {
		return false
	}
	s := strings.ToLower(string(ee.Stderr))
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

func isAlreadyExists(err error) bool { return stderrContains(err, "exists", "duplicate") }

func isNotFound(err error) bool { return stderrContains(err, "no such", "not exist") }

// Захватывает группы "aa:bb:cc:dd:ee:ff" из строк вида
//
//	elements = { aa:bb:..., 11:22:... }
var macInList = regexp.MustCompile(`[0-9a-fA-F]{2}(:[0-9a-fA-F]{2}){5}`)

func parseSetElements(raw []byte) []domain.MAC {
	// Стандартный nft-вывод многострочный — берём всё разом, регексп matchAll.
	matches := macInList.FindAll(bytes.TrimSpace(raw), -1)
	out := make([]domain.MAC, 0, len(matches))
	for _, m := range matches {
		mac, err := domain.NewMAC(string(m))
		if err != nil {
			// Сюда не должны попадать (regexp гарантирует формат), но если
			// конструктор валидации даст otherwise — пропускаем, не падаем.
			continue
		}
		out = append(out, mac)
	}
	return out
}

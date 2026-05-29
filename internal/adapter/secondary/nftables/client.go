// Package nftables — реализация device.NftPort через утилиту `nft` (OpenWrt fw4).
// Adapter работает только с одним сетом MAC-адресов, в который смотрит drop-правило
// `drop ether saddr @banned_macs` (создаётся при bootstrap'е роутера, не ботом).
package nftables

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/tumour/openwrt-bot/internal/adapter/secondary/system"
	"github.com/tumour/openwrt-bot/internal/domain"
)

// Client управляет конкретным nftables-сетом (например `inet fw4 banned_macs`).
// Table и Set вынесены в конструктор, чтобы можно было использовать с разными
// firewall-сетапами (fw4, custom table) без правки кода.
type Client struct {
	runner system.Runner
	table  string // "inet fw4"
	set    string // "banned_macs"
}

func NewClient(runner system.Runner, table, set string) *Client {
	return &Client{runner: runner, table: table, set: set}
}

// AddBanned добавляет MAC в сет. Если уже есть — возвращает domain.ErrAlreadyBanned
// (распознаётся по тексту stderr). Это позволяет use case'у различать классы ошибок.
func (c *Client) AddBanned(ctx context.Context, mac domain.MAC) error {
	_, err := c.runner.Run(ctx, "nft", "add", "element", c.table, c.set,
		fmt.Sprintf("{ %s }", mac))
	if err != nil {
		if isAlreadyExists(err) {
			return fmt.Errorf("%s: %w", mac, domain.ErrAlreadyBanned)
		}
		return fmt.Errorf("nft add element: %w", err)
	}
	return nil
}

// RemoveBanned удаляет MAC из сета. Если не было — domain.ErrNotBanned.
func (c *Client) RemoveBanned(ctx context.Context, mac domain.MAC) error {
	_, err := c.runner.Run(ctx, "nft", "delete", "element", c.table, c.set,
		fmt.Sprintf("{ %s }", mac))
	if err != nil {
		if isNotFound(err) {
			return fmt.Errorf("%s: %w", mac, domain.ErrNotBanned)
		}
		return fmt.Errorf("nft delete element: %w", err)
	}
	return nil
}

// ListBanned читает сет и возвращает список MAC. Парсит человеко-читаемый вывод
// `nft list set` — JSON-режим `nft -j` мог бы быть удобнее, но не во всех версиях
// nftables на OpenWrt он стабилен, поэтому держим текстовый.
func (c *Client) ListBanned(ctx context.Context) ([]domain.MAC, error) {
	out, err := c.runner.Run(ctx, "nft", "list", "set", c.table, c.set)
	if err != nil {
		return nil, fmt.Errorf("nft list set: %w", err)
	}
	return parseSetElements(out)
}

// nft пишет ошибки коннект-уникализации в stderr. Сообщения относительно
// стабильны между версиями, но завязываемся на ключевые слова, не на полный текст.
func isAlreadyExists(err error) bool {
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "exists") || strings.Contains(s, "duplicate")
}

func isNotFound(err error) bool {
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "no such") || strings.Contains(s, "not exist")
}

// Захватывает группы "aa:bb:cc:dd:ee:ff" из строк вида
//   elements = { aa:bb:..., 11:22:... }
var macInList = regexp.MustCompile(`[0-9a-fA-F]{2}(:[0-9a-fA-F]{2}){5}`)

func parseSetElements(raw []byte) ([]domain.MAC, error) {
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
	return out, nil
}

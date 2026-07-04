package nftables

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/tumour/openwrt-bot/internal/adapter/secondary/system"
	"github.com/tumour/openwrt-bot/internal/domain"
)

// RateLimiter реализует device.RateLimitPort поверх отдельной netdev-таблицы
// (например `netdev openwrt_bot`). Скелет — таблицу, map'ы rate_ul/rate_dl и
// policing-цепочки на br-lan — создаёт bootstrap в init.d; бот управляет только
// именованными limit-объектами lim_{ul,dl}_<12hex> и элементами map'ов.
//
// Все мутации — одна атомарная nft-транзакция (semicolon-joined script одним
// аргументом; шелла нет, экранирование не нужно). `destroy` = delete-if-exists,
// поэтому Set (создать-или-обновить) и Remove идемпотентны by construction и
// самовосстанавливают частичное состояние после ручного вмешательства.
type RateLimiter struct {
	runner system.Runner
	table  string // "netdev openwrt_bot"
}

func NewRateLimiter(runner system.Runner, table string) *RateLimiter {
	return &RateLimiter{runner: runner, table: table}
}

// Имена map'ов и префиксы limit-объектов — часть схемы, создаваемой init.d.
// Схема имён неотделима от адаптера, параметризовать её — ложная гибкость.
const (
	rateMapUL   = "rate_ul"
	rateMapDL   = "rate_dl"
	limPrefixUL = "lim_ul_"
	limPrefixDL = "lim_dl_"
)

// Set устанавливает или обновляет лимит: N КБ/с на каждое направление
// независимо. Порядок внутри транзакции обязателен: снять ссылки из map'ов →
// удалить старые limit-объекты → создать новые → сослаться из map'ов.
func (c *RateLimiter) Set(ctx context.Context, mac domain.MAC, rate domain.Rate) error {
	h := macHex(mac)
	ul, dl := limPrefixUL+h, limPrefixDL+h
	burst := burstKB(rate)
	script := strings.Join([]string{
		fmt.Sprintf("destroy element %s %s { %s }", c.table, rateMapUL, mac),
		fmt.Sprintf("destroy element %s %s { %s }", c.table, rateMapDL, mac),
		fmt.Sprintf("destroy limit %s %s", c.table, ul),
		fmt.Sprintf("destroy limit %s %s", c.table, dl),
		fmt.Sprintf("add limit %s %s { rate over %d kbytes/second burst %d kbytes ; }", c.table, ul, rate.KBps(), burst),
		fmt.Sprintf("add limit %s %s { rate over %d kbytes/second burst %d kbytes ; }", c.table, dl, rate.KBps(), burst),
		fmt.Sprintf(`add element %s %s { %s : "%s" }`, c.table, rateMapUL, mac, ul),
		fmt.Sprintf(`add element %s %s { %s : "%s" }`, c.table, rateMapDL, mac, dl),
	}, "; ")
	if _, err := c.runner.Run(ctx, "nft", script); err != nil {
		return fmt.Errorf("nft set rate limit: %w", err)
	}
	return nil
}

// Remove снимает лимит. Отсутствующий лимит — no-op (destroy не ошибается),
// частично снятый (после ручного вмешательства) — дочищается.
func (c *RateLimiter) Remove(ctx context.Context, mac domain.MAC) error {
	h := macHex(mac)
	script := strings.Join([]string{
		fmt.Sprintf("destroy element %s %s { %s }", c.table, rateMapUL, mac),
		fmt.Sprintf("destroy element %s %s { %s }", c.table, rateMapDL, mac),
		fmt.Sprintf("destroy limit %s %s", c.table, limPrefixUL+h),
		fmt.Sprintf("destroy limit %s %s", c.table, limPrefixDL+h),
	}, "; ")
	if _, err := c.runner.Run(ctx, "nft", script); err != nil {
		return fmt.Errorf("nft remove rate limit: %w", err)
	}
	return nil
}

// List читает текущие лимиты из вывода `nft list table` (текстовый режим —
// та же причина, что у Client.List). MAC восстанавливается из имени
// limit-объекта; читаем только dl-объекты — ul по построению идентичен.
func (c *RateLimiter) List(ctx context.Context) (map[domain.MAC]domain.Rate, error) {
	out, err := c.runner.Run(ctx, "nft", "list", "table", c.table)
	if err != nil {
		return nil, fmt.Errorf("nft list table: %w", err)
	}
	return parseRateLimits(out), nil
}

// Захватывает имя dl-limit-объекта и его rate из многострочного вывода вида
//
//	limit lim_dl_aabbcc112233 {
//		rate over 1 mbytes/second burst 512 kbytes
//	}
//
// nft укрупняет единицы при печати (1024 kbytes → 1 mbytes), поэтому регексп
// обязан покрывать все байтовые единицы, а парсер — приводить их к КБ.
var limitDLRe = regexp.MustCompile(`limit ` + limPrefixDL + `([0-9a-f]{12})\s*\{\s*rate over (\d+) (bytes|kbytes|mbytes|gbytes)/second`)

func parseRateLimits(raw []byte) map[domain.MAC]domain.Rate {
	matches := limitDLRe.FindAllStringSubmatch(string(raw), -1)
	out := make(map[domain.MAC]domain.Rate, len(matches))
	for _, m := range matches {
		mac, err := hexToMAC(m[1])
		if err != nil {
			continue // насоздавали руками мимо схемы — пропускаем, не падаем
		}
		n, err := strconv.Atoi(m[2])
		if err != nil {
			continue
		}
		rate, err := domain.NewRate(n * unitKB(m[3]))
		if err != nil {
			continue
		}
		out[mac] = rate
	}
	return out
}

// unitKB — множитель байтовой единицы nft-вывода к КБ. bytes → 0: лимит меньше
// килобайта ботом не создаётся, такую ручную запись пропускаем (NewRate(0) невалиден).
func unitKB(unit string) int {
	switch unit {
	case "kbytes":
		return 1
	case "mbytes":
		return 1024
	case "gbytes":
		return 1024 * 1024
	default:
		return 0
	}
}

// burstKB — размер token-bucket burst для лимита: полсекунды трафика,
// clamp [16..2048] КБ. Burst около нуля жёстко дропает пакетные пачки TCP и
// роняет фактическую скорость сильно ниже планки; нижняя граница ~10 полных
// ethernet-кадров, верхняя — чтобы большие лимиты не размывались в коротких
// замерах. Значение подобрано умозрительно — tunable по итогам замеров.
func burstKB(rate domain.Rate) int {
	b := rate.KBps() / 2
	if b < 16 {
		return 16
	}
	if b > 2048 {
		return 2048
	}
	return b
}

// macHex — MAC без разделителей для имени nft-объекта: aa:bb:cc:11:22:33 → aabbcc112233.
func macHex(mac domain.MAC) string {
	return strings.ReplaceAll(mac.String(), ":", "")
}

// hexToMAC — обратно: aabbcc112233 → aa:bb:cc:11:22:33 (через валидацию VO).
func hexToMAC(hex12 string) (domain.MAC, error) {
	if len(hex12) != 12 {
		return "", fmt.Errorf("%w: %q", domain.ErrInvalidMAC, hex12)
	}
	parts := make([]string, 0, 6)
	for i := 0; i < 12; i += 2 {
		parts = append(parts, hex12[i:i+2])
	}
	return domain.NewMAC(strings.Join(parts, ":"))
}

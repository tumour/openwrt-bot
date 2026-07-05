package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"sync/atomic"
	"time"

	tele "gopkg.in/telebot.v3"
)

// resilientPoller — long-poller с экспоненциальным backoff и state-transition
// логами. Замена tele.LongPoller, у которого два дефекта при недоступном
// Telegram (лежащий VPN-socks): мгновенный retry без паузы — busy-loop на CPU
// роутера при connection refused — и полная немота (ошибка логируется только
// при Verbose). Заодно чинит унаследованный дедлок: LongPoller шлёт в dest
// безусловно и может навсегда заблокировать Stop.
type resilientPoller struct {
	timeout time.Duration // server-side long-poll timeout для getUpdates
	// lastUpdateID читается/пишется только горутиной Poll (как у LongPoller) —
	// мьютекс не нужен.
	lastUpdateID int
	backoff      backoff // тесты инжектят малые значения
	// connected — состояние Telegram-канала, принадлежит telegram.Bot (его API
	// Connected); поллер лишь переключает его в state-transition точках.
	connected *atomic.Bool
	logger    *slog.Logger
}

func newResilientPoller(logger *slog.Logger, connected *atomic.Bool) *resilientPoller {
	return &resilientPoller{
		timeout:   10 * time.Second,
		backoff:   newBackoff(),
		connected: connected,
		logger:    logger,
	}
}

// Poll реализует tele.Poller: крутит getUpdates до закрытия stop.
func (p *resilientPoller) Poll(b *tele.Bot, dest chan tele.Update, stop chan struct{}) {
	var offline, flooding bool // эпизоды: state-transition логи, не каждая итерация
	for {
		select {
		case <-stop:
			return
		default:
		}

		updates, err := p.getUpdates(b)
		if err != nil {
			// Отмена Raw через stopClient = идёт Stop(): не логируем и не спим,
			// ждём закрытия stop (Start закроет его сразу после confirm).
			// Страховочный time.After — чтобы теоретическая отмена не из Stop
			// не превратилась в busy-loop.
			if errors.Is(err, context.Canceled) {
				select {
				case <-stop:
					return
				case <-time.After(time.Second):
				}
				continue
			}

			// 429 — канал жив (сервер ответил), Telegram лишь просит паузу:
			// это не оффлайн, а если мы БЫЛИ в оффлайне — это восстановление,
			// и сетевой backoff больше неактуален (иначе эскалированные 60s
			// пережили бы восстановление и растянули следующий свежий обрыв).
			var flood tele.FloodError
			if errors.As(err, &flood) && flood.RetryAfter > 0 {
				if offline {
					p.connected.Store(true)
					p.logger.Info("telegram снова доступен")
					offline = false
				}
				p.backoff.reset()
				delay := time.Duration(flood.RetryAfter) * time.Second
				// Как и оффлайн: Info один раз на эпизод, повторы — Debug,
				// иначе затяжной шторм 429 вымывает кольцо logread.
				if !flooding {
					p.logger.Info("telegram rate limit, жду", "retry_after", delay)
					flooding = true
				} else {
					p.logger.Debug("telegram всё ещё rate limit", "retry_after", delay)
				}
				select {
				case <-stop:
					return
				case <-time.After(delay):
				}
				continue
			}
			flooding = false

			delay := p.backoff.next()
			// State-transition: Warn один раз при уходе в оффлайн, повторы —
			// Debug, иначе ночь без VPN вымывает 64КБ-кольцо logread.
			if !offline {
				p.connected.Store(false)
				p.logger.Warn("telegram недоступен, ухожу в retry", "err", err)
				offline = true
			} else {
				p.logger.Debug("telegram всё ещё недоступен", "err", err, "retry_in", delay)
			}
			select {
			case <-stop:
				return
			case <-time.After(delay):
			}
			continue
		}

		if offline {
			p.connected.Store(true)
			p.logger.Info("telegram снова доступен")
			offline = false
		}
		flooding = false
		p.backoff.reset()

		for _, u := range updates {
			p.lastUpdateID = u.ID
			select {
			case dest <- u:
			case <-stop:
				// Недоставленный апдейт не потерян: offset подтверждается только
				// следующим getUpdates — после рестарта Telegram передоставит его.
				return
			}
		}
	}
}

// getUpdates — паритет с telebot getUpdates (api.go): те же параметры и формат
// ответа. allowed_updates не передаём — эквивалент nil-слайса у LongPoller
// (Telegram использует дефолтный набор).
func (p *resilientPoller) getUpdates(b *tele.Bot) ([]tele.Update, error) {
	params := map[string]string{
		"offset":  strconv.Itoa(p.lastUpdateID + 1),
		"timeout": strconv.Itoa(int(p.timeout / time.Second)),
	}
	data, err := b.Raw("getUpdates", params)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Result []tele.Update
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.Result, nil
}

package domain

import "fmt"

// Action — операция над устройством, которую может выполнить планировщик при
// срабатывании таймера (или бот немедленно). Значения 1:1 соответствуют use cases
// device.* (Ban/Unban/DisableVPN/EnableVPN) — их и вызывают, поэтому правило
// «повтор = no-op» живёт в одном месте и не дублируется.
type Action uint8

const (
	ActionBan    Action = iota + 1 // выключить интернет (бан)
	ActionUnban                    // включить интернет (разбан)
	ActionVPNOff                   // пустить мимо VPN
	ActionVPNOn                    // вернуть в VPN
)

// String — стабильный идентификатор действия: тот же токен, что и имя команды
// (/ban, /vpnoff…). Идёт в payload callback-кнопок, в аргумент /timer и в логи;
// человекочитаемые русские подписи — забота presenter.
func (a Action) String() string {
	switch a {
	case ActionBan:
		return "ban"
	case ActionUnban:
		return "unban"
	case ActionVPNOff:
		return "vpnoff"
	case ActionVPNOn:
		return "vpnon"
	default:
		return "unknown"
	}
}

// ParseAction — обратное к String: разбирает токен действия (из callback-кнопки
// или аргумента /timer). Неизвестный токен → ErrInvalidAction.
func ParseAction(s string) (Action, error) {
	switch s {
	case "ban":
		return ActionBan, nil
	case "unban":
		return ActionUnban, nil
	case "vpnoff":
		return ActionVPNOff, nil
	case "vpnon":
		return ActionVPNOn, nil
	default:
		return 0, fmt.Errorf("%w: %q", ErrInvalidAction, s)
	}
}

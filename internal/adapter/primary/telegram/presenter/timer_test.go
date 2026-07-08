package presenter

import (
	"strings"
	"testing"
	"time"

	"github.com/tumour/openwrt-bot/internal/app/timer"
	"github.com/tumour/openwrt-bot/internal/domain"
)

func TestRemainingText(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "вот-вот"},
		{time.Minute, "через 1 мин"},
		{42 * time.Minute, "через 42 мин"},
		{time.Hour, "через 1 ч 00 мин"},
		{65 * time.Minute, "через 1 ч 05 мин"},
		{4 * time.Hour, "через 4 ч 00 мин"},
	}
	for _, c := range cases {
		if got := RemainingText(c.d); got != c.want {
			t.Errorf("RemainingText(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestMinutesLabel(t *testing.T) {
	cases := map[int]string{
		5:   "5 мин",
		30:  "30 мин",
		60:  "1 ч",
		120: "2 ч",
		90:  "1 ч 30 мин",
	}
	for mins, want := range cases {
		if got := MinutesLabel(mins); got != want {
			t.Errorf("MinutesLabel(%d) = %q, want %q", mins, got, want)
		}
	}
}

func TestActionLabelAndButton_CoverAllActions(t *testing.T) {
	for _, a := range []domain.Action{domain.ActionBan, domain.ActionUnban, domain.ActionVPNOff, domain.ActionVPNOn} {
		if ActionLabel(a) == "?" {
			t.Errorf("ActionLabel(%v) не задан", a)
		}
		if strings.Contains(ActionButton(a), "?") {
			t.Errorf("ActionButton(%v) не задан", a)
		}
	}
}

func TestTimers_Empty(t *testing.T) {
	got := Timers(nil, nil)
	if !strings.Contains(got, "Активных нет") {
		t.Errorf("пустой экран не сообщает об отсутствии таймеров: %q", got)
	}
}

func TestTimers_EscapesDeviceNameAndShowsRemaining(t *testing.T) {
	mac := mustMAC(t, "aa:bb:cc:11:22:33")
	tasks := []timer.View{{
		Job:       domain.DeviceJob{MAC: mac, Action: domain.ActionBan},
		Remaining: 42 * time.Minute,
	}}
	names := map[domain.MAC]string{mac: "<b>hax</b>"}

	got := Timers(tasks, names)
	if strings.Contains(got, "<b>hax</b>") {
		t.Error("имя устройства должно экранироваться")
	}
	if !strings.Contains(got, "&lt;b&gt;hax&lt;/b&gt;") {
		t.Errorf("ждём экранированное имя, got %q", got)
	}
	if !strings.Contains(got, "через 42 мин") {
		t.Errorf("нет остатка времени в строке: %q", got)
	}
}

func TestTimers_UnknownMACFallsBackToMAC(t *testing.T) {
	mac := mustMAC(t, "aa:bb:cc:11:22:33")
	tasks := []timer.View{{
		Job:       domain.DeviceJob{MAC: mac, Action: domain.ActionUnban},
		Remaining: time.Minute,
	}}

	got := Timers(tasks, nil) // имени нет — показываем MAC
	if !strings.Contains(got, "aa:bb:cc:11:22:33") {
		t.Errorf("ждём MAC как запасное имя, got %q", got)
	}
}

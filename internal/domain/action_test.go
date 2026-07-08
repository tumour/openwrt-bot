package domain

import (
	"errors"
	"testing"
)

// String ↔ ParseAction — round-trip: токены совпадают с именами команд
// (/ban, /vpnoff…), поэтому важно, что они стабильны.
func TestAction_StringParseRoundTrip(t *testing.T) {
	for _, a := range []Action{ActionBan, ActionUnban, ActionVPNOff, ActionVPNOn} {
		got, err := ParseAction(a.String())
		if err != nil {
			t.Fatalf("ParseAction(%q): unexpected error: %v", a.String(), err)
		}
		if got != a {
			t.Errorf("round-trip %q: got %d, want %d", a.String(), got, a)
		}
	}
}

func TestAction_String_Tokens(t *testing.T) {
	want := map[Action]string{
		ActionBan:    "ban",
		ActionUnban:  "unban",
		ActionVPNOff: "vpnoff",
		ActionVPNOn:  "vpnon",
	}
	for a, s := range want {
		if got := a.String(); got != s {
			t.Errorf("Action(%d).String() = %q, want %q", a, got, s)
		}
	}
	if got := Action(0).String(); got != "unknown" {
		t.Errorf("zero Action.String() = %q, want %q", got, "unknown")
	}
}

func TestParseAction_Invalid(t *testing.T) {
	for _, s := range []string{"", "BAN", "reboot", "ban ", "vpn"} {
		_, err := ParseAction(s)
		if !errors.Is(err, ErrInvalidAction) {
			t.Errorf("ParseAction(%q) err = %v, want ErrInvalidAction", s, err)
		}
	}
}

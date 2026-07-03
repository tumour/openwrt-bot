package domain

import (
	"errors"
	"sort"
	"testing"
)

func TestNewPermission(t *testing.T) {
	for _, known := range AllPermissions() {
		p, err := NewPermission(string(known))
		if err != nil {
			t.Errorf("NewPermission(%q): %v", known, err)
		}
		if p != known {
			t.Errorf("got %q, want %q", p, known)
		}
	}
	if _, err := NewPermission("fly_to_moon"); !errors.Is(err, ErrUnknownPermission) {
		t.Errorf("err = %v, want ErrUnknownPermission", err)
	}
	if _, err := NewPermission(""); !errors.Is(err, ErrUnknownPermission) {
		t.Errorf("err = %v, want ErrUnknownPermission", err)
	}
}

func TestAllPermissions(t *testing.T) {
	all := AllPermissions()
	if len(all) != len(knownPermissions) {
		t.Fatalf("AllPermissions вернул %d прав, в каталоге %d", len(all), len(knownPermissions))
	}
	// Детерминированный порядок — на нём строится сравнение ролей и seed.
	if !sort.SliceIsSorted(all, func(i, j int) bool { return all[i] < all[j] }) {
		t.Error("AllPermissions должен быть отсортирован")
	}
}

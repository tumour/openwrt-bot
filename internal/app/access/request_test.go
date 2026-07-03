package access

import (
	"context"
	"errors"
	"testing"

	"github.com/tumour/openwrt-bot/internal/domain"
)

func TestRequestAccess_Execute_CreatesPendingAndListsAdmins(t *testing.T) {
	store := defaultRolesStore()
	uc := NewRequestAccess(store.Users(), store.Roles())

	out, err := uc.Execute(context.Background(), RequestAccessInput{UserID: 42, Name: "Батя"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Created {
		t.Fatal("Created = false, want true")
	}
	if out.User.Status != domain.StatusPending {
		t.Errorf("status = %q, want pending", out.User.Status)
	}
	if len(out.Admins) != 1 || out.Admins[0].ID != 1 {
		t.Errorf("Admins = %v, want [admin id=1]", out.Admins)
	}
	if stored, err := store.Users().Get(context.Background(), 42); err != nil || stored.Status != domain.StatusPending {
		t.Errorf("заявка не сохранилась: %v / %v", stored, err)
	}
}

// Дедуп: повторный /start — и от pending, и от активного — не создаёт заявку
// и не даёт повода спамить админов.
func TestRequestAccess_Execute_Dedup(t *testing.T) {
	store := defaultRolesStore(domain.NewPendingUser(42, "Батя"))
	uc := NewRequestAccess(store.Users(), store.Roles())

	for _, id := range []int64{42, 1} { // 42 — pending, 1 — активный админ
		out, err := uc.Execute(context.Background(), RequestAccessInput{UserID: id, Name: "x"})
		if err != nil {
			t.Fatalf("id %d: unexpected error: %v", id, err)
		}
		if out.Created {
			t.Errorf("id %d: Created = true, want false (дедуп)", id)
		}
	}
}

func TestRequestAccess_Execute_InvalidID(t *testing.T) {
	store := defaultRolesStore()
	uc := NewRequestAccess(store.Users(), store.Roles())

	if _, err := uc.Execute(context.Background(), RequestAccessInput{UserID: -5}); !errors.Is(err, domain.ErrInvalidUserID) {
		t.Errorf("err = %v, want ErrInvalidUserID", err)
	}
}

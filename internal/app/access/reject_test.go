package access

import (
	"context"
	"errors"
	"testing"

	"github.com/tumour/openwrt-bot/internal/domain"
)

func TestReject_Execute_OK(t *testing.T) {
	store := defaultRolesStore(domain.NewPendingUser(42, "Батя"))
	uc := NewReject(store)

	out, err := uc.Execute(context.Background(), RejectInput{ActorID: 1, TargetID: 42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.User.ID != 42 {
		t.Errorf("отклонённый = %+v, want id=42", out.User)
	}
	if _, err := store.Users().Get(context.Background(), 42); !errors.Is(err, domain.ErrUserNotFound) {
		t.Error("заявка должна быть удалена — человек сможет попроситься снова")
	}
}

func TestReject_Execute_ActiveTarget(t *testing.T) {
	store := defaultRolesStore(domain.NewActiveUser(2, "гость", domain.RoleUser))
	uc := NewReject(store)

	if _, err := uc.Execute(context.Background(), RejectInput{ActorID: 1, TargetID: 2}); !errors.Is(err, domain.ErrNotPending) {
		t.Errorf("err = %v, want ErrNotPending (активных удаляют через RemoveUser)", err)
	}
}

func TestReject_Execute_Forbidden(t *testing.T) {
	store := defaultRolesStore(
		domain.NewActiveUser(2, "гость", domain.RoleUser),
		domain.NewPendingUser(42, "Батя"),
	)
	uc := NewReject(store)

	if _, err := uc.Execute(context.Background(), RejectInput{ActorID: 2, TargetID: 42}); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("err = %v, want ErrForbidden", err)
	}
}

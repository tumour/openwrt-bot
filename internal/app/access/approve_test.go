package access

import (
	"context"
	"errors"
	"testing"

	"github.com/tumour/openwrt-bot/internal/domain"
)

func TestApprove_Execute_OK(t *testing.T) {
	store := defaultRolesStore(domain.NewPendingUser(42, "Батя"))
	uc := NewApprove(store)

	out, err := uc.Execute(context.Background(), ApproveInput{ActorID: 1, TargetID: 42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.User.IsActive() || out.User.Role != domain.RoleUser {
		t.Errorf("одобренный = %+v, want active с ролью user", out.User)
	}
	stored, _ := store.Users().Get(context.Background(), 42)
	if !stored.IsActive() {
		t.Error("статус в сторе не обновился")
	}
}

func TestApprove_Execute_Forbidden(t *testing.T) {
	store := defaultRolesStore(
		domain.NewActiveUser(2, "гость", domain.RoleUser),
		domain.NewPendingUser(3, "заявка"),
		domain.NewPendingUser(42, "Батя"),
	)
	uc := NewApprove(store)

	// Актор без manage_users и актор-pending — оба запрещены.
	for _, actor := range []domain.UserID{2, 3, 99} {
		if _, err := uc.Execute(context.Background(), ApproveInput{ActorID: actor, TargetID: 42}); !errors.Is(err, domain.ErrForbidden) {
			t.Errorf("actor %d: err = %v, want ErrForbidden", actor, err)
		}
	}
}

func TestApprove_Execute_TargetErrors(t *testing.T) {
	store := defaultRolesStore(domain.NewActiveUser(2, "уже активен", domain.RoleUser))
	uc := NewApprove(store)

	if _, err := uc.Execute(context.Background(), ApproveInput{ActorID: 1, TargetID: 99}); !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("нет цели: err = %v, want ErrUserNotFound", err)
	}
	if _, err := uc.Execute(context.Background(), ApproveInput{ActorID: 1, TargetID: 2}); !errors.Is(err, domain.ErrNotPending) {
		t.Errorf("цель активна: err = %v, want ErrNotPending", err)
	}
}

// Одобрение не должно порождать юзера с висячей ролью: нет роли user — ошибка.
func TestApprove_Execute_DefaultRoleMissing(t *testing.T) {
	admin := domain.NewActiveUser(1, "admin", domain.RoleAdmin)
	adminRole := domain.Role{Name: domain.RoleAdmin, Permissions: domain.AllPermissions()}
	store := newMemStore(
		[]domain.User{admin, domain.NewPendingUser(42, "Батя")},
		[]domain.Role{adminRole}, // роли user нет
	)
	uc := NewApprove(store)

	if _, err := uc.Execute(context.Background(), ApproveInput{ActorID: 1, TargetID: 42}); !errors.Is(err, domain.ErrRoleNotFound) {
		t.Errorf("err = %v, want ErrRoleNotFound", err)
	}
	if stored, _ := store.Users().Get(context.Background(), 42); stored.IsActive() {
		t.Error("юзер не должен был активироваться без роли")
	}
}

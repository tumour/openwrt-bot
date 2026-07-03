package access

import (
	"context"
	"errors"
	"testing"

	"github.com/tumour/openwrt-bot/internal/domain"
)

func TestSetRole_Execute_Promote(t *testing.T) {
	store := defaultRolesStore(domain.NewActiveUser(2, "гость", domain.RoleUser))
	uc := NewSetRole(store.Users(), store.Roles())

	out, err := uc.Execute(context.Background(), SetRoleInput{ActorID: 1, TargetID: 2, Role: domain.RoleAdmin})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.User.Role != domain.RoleAdmin {
		t.Errorf("роль = %q, want admin", out.User.Role)
	}
}

// Guard последнего админа: единственного носителя manage_users разжаловать
// нельзя, а одного из двух — можно.
func TestSetRole_Execute_LastAdminGuard(t *testing.T) {
	store := defaultRolesStore() // один админ id=1
	uc := NewSetRole(store.Users(), store.Roles())

	if _, err := uc.Execute(context.Background(), SetRoleInput{ActorID: 1, TargetID: 1, Role: domain.RoleUser}); !errors.Is(err, domain.ErrLastAdmin) {
		t.Errorf("err = %v, want ErrLastAdmin", err)
	}

	// Второй админ появился — теперь разжалование первого легально.
	second := domain.NewActiveUser(5, "второй", domain.RoleAdmin)
	if err := store.Users().Put(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	if _, err := uc.Execute(context.Background(), SetRoleInput{ActorID: 5, TargetID: 1, Role: domain.RoleUser}); err != nil {
		t.Errorf("с двумя админами разжалование должно пройти: %v", err)
	}
}

func TestSetRole_Execute_Errors(t *testing.T) {
	store := defaultRolesStore(
		domain.NewActiveUser(2, "гость", domain.RoleUser),
		domain.NewPendingUser(3, "заявка"),
	)
	uc := NewSetRole(store.Users(), store.Roles())
	ctx := context.Background()

	if _, err := uc.Execute(ctx, SetRoleInput{ActorID: 1, TargetID: 2, Role: "ghost"}); !errors.Is(err, domain.ErrRoleNotFound) {
		t.Errorf("несуществующая роль: err = %v, want ErrRoleNotFound", err)
	}
	if _, err := uc.Execute(ctx, SetRoleInput{ActorID: 1, TargetID: 3, Role: domain.RoleUser}); !errors.Is(err, domain.ErrNotActive) {
		t.Errorf("pending-цель: err = %v, want ErrNotActive", err)
	}
	if _, err := uc.Execute(ctx, SetRoleInput{ActorID: 2, TargetID: 2, Role: domain.RoleAdmin}); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("актор без прав: err = %v, want ErrForbidden", err)
	}
}

func TestSetRole_Execute_SameRole_IsNoOp(t *testing.T) {
	store := defaultRolesStore(domain.NewActiveUser(2, "гость", domain.RoleUser))
	uc := NewSetRole(store.Users(), store.Roles())

	out, err := uc.Execute(context.Background(), SetRoleInput{ActorID: 1, TargetID: 2, Role: domain.RoleUser})
	if err != nil {
		t.Fatalf("та же роль должна быть no-op: %v", err)
	}
	if out.User.Role != domain.RoleUser {
		t.Errorf("роль = %q, want user", out.User.Role)
	}
}

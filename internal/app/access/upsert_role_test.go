package access

import (
	"context"
	"errors"
	"testing"

	"github.com/tumour/openwrt-bot/internal/domain"
)

func TestUpsertRole_Execute_CreatesRole(t *testing.T) {
	store := defaultRolesStore()
	uc := NewUpsertRole(store.Users(), store.Roles())

	out, err := uc.Execute(context.Background(), UpsertRoleInput{
		ActorID:     1,
		Name:        "moderator",
		Permissions: []string{"ban_devices", "view_status"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Role.Has(domain.PermBanDevices) || out.Role.Has(domain.PermManageUsers) {
		t.Errorf("права роли = %v", out.Role.Permissions)
	}
	if _, err := store.Roles().Get(context.Background(), "moderator"); err != nil {
		t.Errorf("роль не сохранилась: %v", err)
	}
}

func TestUpsertRole_Execute_UnknownPermission(t *testing.T) {
	store := defaultRolesStore()
	uc := NewUpsertRole(store.Users(), store.Roles())

	if _, err := uc.Execute(context.Background(), UpsertRoleInput{ActorID: 1, Name: "x", Permissions: []string{"fly_to_moon"}}); !errors.Is(err, domain.ErrUnknownPermission) {
		t.Errorf("err = %v, want ErrUnknownPermission", err)
	}
}

// Guard: снятие manage_users с роли admin, когда все админы сидят на ней, —
// бот остался бы неуправляемым. Но при живом носителе на ДРУГОЙ роли — можно.
func TestUpsertRole_Execute_LastAdminGuard(t *testing.T) {
	store := defaultRolesStore()
	uc := NewUpsertRole(store.Users(), store.Roles())
	ctx := context.Background()

	in := UpsertRoleInput{ActorID: 1, Name: string(domain.RoleAdmin), Permissions: []string{"view_status"}}
	if _, err := uc.Execute(ctx, in); !errors.Is(err, domain.ErrLastAdmin) {
		t.Fatalf("err = %v, want ErrLastAdmin", err)
	}

	// Носитель manage_users на другой роли: правка admin теперь легальна.
	super := domain.Role{Name: "superuser", Permissions: []domain.Permission{domain.PermManageUsers}}
	if err := store.Roles().Put(ctx, super); err != nil {
		t.Fatal(err)
	}
	if err := store.Users().Put(ctx, domain.NewActiveUser(7, "супер", "superuser")); err != nil {
		t.Fatal(err)
	}
	if _, err := uc.Execute(ctx, in); err != nil {
		t.Errorf("с носителем на другой роли правка должна пройти: %v", err)
	}
}

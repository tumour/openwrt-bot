package access

import (
	"context"
	"errors"
	"testing"

	"github.com/tumour/openwrt-bot/internal/domain"
)

func TestListUsers_Execute_PendingFirst(t *testing.T) {
	store := defaultRolesStore(
		domain.NewActiveUser(2, "гость", domain.RoleUser),
		domain.NewPendingUser(9, "поздняя заявка"),
		domain.NewPendingUser(3, "ранняя заявка"),
	)
	uc := NewListUsers(store.Users(), store.Roles())

	out, err := uc.Execute(context.Background(), ListUsersInput{ActorID: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantOrder := []domain.UserID{3, 9, 1, 2} // pending по ID, затем active по ID
	if len(out.Users) != len(wantOrder) {
		t.Fatalf("users = %d, want %d", len(out.Users), len(wantOrder))
	}
	for i, want := range wantOrder {
		if out.Users[i].ID != want {
			t.Errorf("users[%d].ID = %d, want %d", i, out.Users[i].ID, want)
		}
	}
}

func TestListRoles_Execute_Sorted(t *testing.T) {
	store := defaultRolesStore()
	uc := NewListRoles(store.Users(), store.Roles())

	out, err := uc.Execute(context.Background(), ListRolesInput{ActorID: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Roles) != 2 || out.Roles[0].Name != domain.RoleAdmin || out.Roles[1].Name != domain.RoleUser {
		t.Errorf("roles = %v, want [admin user]", out.Roles)
	}
}

func TestList_Forbidden(t *testing.T) {
	store := defaultRolesStore(domain.NewActiveUser(2, "гость", domain.RoleUser))
	ctx := context.Background()

	if _, err := NewListUsers(store.Users(), store.Roles()).Execute(ctx, ListUsersInput{ActorID: 2}); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("ListUsers: err = %v, want ErrForbidden", err)
	}
	if _, err := NewListRoles(store.Users(), store.Roles()).Execute(ctx, ListRolesInput{ActorID: 2}); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("ListRoles: err = %v, want ErrForbidden", err)
	}
}

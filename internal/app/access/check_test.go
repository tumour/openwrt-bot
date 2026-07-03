package access

import (
	"context"
	"testing"

	"github.com/tumour/openwrt-bot/internal/domain"
)

func TestCheck_Execute(t *testing.T) {
	store := defaultRolesStore(
		domain.NewActiveUser(2, "гость", domain.RoleUser),
		domain.NewPendingUser(3, "заявка"),
		domain.NewActiveUser(4, "сирота", "ghost_role"), // роль удалили руками
	)
	uc := NewCheck(store.Users(), store.Roles())

	tests := []struct {
		name        string
		id          int64
		wantAllowed bool
		wantManage  bool // есть ли manage_users в гранте
	}{
		{"админ допущен со всеми правами", 1, true, true},
		{"юзер допущен без manage_users", 2, true, false},
		{"pending не допущен", 3, false, false},
		{"активный с несуществующей ролью не допущен", 4, false, false},
		{"неизвестный не допущен", 99, false, false},
		{"невалидный ID (группа) не допущен", -100500, false, false},
		{"нулевой ID не допущен", 0, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := uc.Execute(context.Background(), CheckInput{UserID: tc.id})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out.Allowed != tc.wantAllowed {
				t.Fatalf("Allowed = %v, want %v", out.Allowed, tc.wantAllowed)
			}
			if got := out.Grant.Has(domain.PermManageUsers); got != tc.wantManage {
				t.Errorf("Has(manage_users) = %v, want %v", got, tc.wantManage)
			}
		})
	}
}

func TestCheck_Execute_StoreError_Propagates(t *testing.T) {
	store := defaultRolesStore()
	store.failUsers = context.DeadlineExceeded
	uc := NewCheck(store.Users(), store.Roles())

	if _, err := uc.Execute(context.Background(), CheckInput{UserID: 1}); err == nil {
		t.Error("инфраструктурная ошибка стора должна подниматься, а не глотаться")
	}
}

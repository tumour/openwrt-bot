package domain

import (
	"errors"
	"testing"
)

func TestNewUserID(t *testing.T) {
	tests := []struct {
		name    string
		id      int64
		wantErr error
	}{
		{"valid", 680982436, nil},
		{"zero", 0, ErrInvalidUserID},
		{"negative (группа/канал)", -100123, ErrInvalidUserID},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewUserID(tc.id)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if int64(got) != tc.id {
				t.Errorf("got %d, want %d", got, tc.id)
			}
		})
	}
}

func TestUser_Approve(t *testing.T) {
	u := NewPendingUser(1, "Батя")
	if u.IsActive() {
		t.Fatal("pending не должен быть active")
	}
	if u.Role != "" {
		t.Fatalf("pending должен быть без роли, got %q", u.Role)
	}

	if err := u.Approve(RoleUser); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if !u.IsActive() {
		t.Error("после Approve юзер должен быть active")
	}
	if u.Role != RoleUser {
		t.Errorf("роль = %q, want %q", u.Role, RoleUser)
	}

	// Повторное одобрение — ошибка: перехода active→active нет.
	if err := u.Approve(RoleAdmin); !errors.Is(err, ErrNotPending) {
		t.Errorf("повторный Approve: err = %v, want ErrNotPending", err)
	}
	if u.Role != RoleUser {
		t.Errorf("роль после неудачного Approve не должна меняться, got %q", u.Role)
	}
}

func TestNewActiveUser(t *testing.T) {
	u := NewActiveUser(1, "admin", RoleAdmin)
	if !u.IsActive() {
		t.Error("NewActiveUser должен быть active")
	}
	if u.Role != RoleAdmin {
		t.Errorf("роль = %q, want %q", u.Role, RoleAdmin)
	}
}

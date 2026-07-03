package domain

import (
	"errors"
	"testing"
)

func TestNewRole(t *testing.T) {
	tests := []struct {
		name    string
		role    string
		perms   []string
		want    int // сколько прав ожидаем после схлопывания дубликатов
		wantErr error
	}{
		{"valid with perms", "moderator", []string{"ban_devices", "view_status"}, 2, nil},
		{"valid without perms", "guest", nil, 0, nil},
		{"duplicates collapse", "dup", []string{"view_status", "view_status"}, 1, nil},
		{"unknown permission", "bad", []string{"fly_to_moon"}, 0, ErrUnknownPermission},
		{"empty name", "", nil, 0, ErrInvalidRoleName},
		{"uppercase name", "Admin", nil, 0, ErrInvalidRoleName},
		{"name with space", "power user", nil, 0, ErrInvalidRoleName},
		{"name too long", "a_role_name_that_is_way_too_long_to_fit", nil, 0, ErrInvalidRoleName},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewRole(tc.role, tc.perms)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got.Permissions) != tc.want {
				t.Errorf("permissions = %d, want %d", len(got.Permissions), tc.want)
			}
		})
	}
}

func TestRole_Has(t *testing.T) {
	r := Role{Name: "x", Permissions: []Permission{PermViewStatus}}
	if !r.Has(PermViewStatus) {
		t.Error("Has(PermViewStatus) = false, want true")
	}
	if r.Has(PermManageUsers) {
		t.Error("Has(PermManageUsers) = true, want false")
	}
}

func TestRole_Grant(t *testing.T) {
	r := Role{Name: "x"}
	if !r.Grant(PermViewStatus) {
		t.Error("первый Grant должен вернуть true")
	}
	if r.Grant(PermViewStatus) {
		t.Error("повторный Grant должен вернуть false (идемпотентность)")
	}
	if !r.Has(PermViewStatus) {
		t.Error("после Grant право должно присутствовать")
	}
}

// TestDefaultRoles закрепляет ролевую политику бота: admin владеет всем
// каталогом, user — только просмотром (status/list/speedtest), управление
// устройствами и юзерами у него отсутствует.
func TestDefaultRoles(t *testing.T) {
	roles := DefaultRoles()
	byName := make(map[RoleName]Role, len(roles))
	for _, r := range roles {
		byName[r.Name] = r
	}

	admin, ok := byName[RoleAdmin]
	if !ok {
		t.Fatal("нет встроенной роли admin")
	}
	for _, p := range AllPermissions() {
		if !admin.Has(p) {
			t.Errorf("admin должен иметь %q", p)
		}
	}

	user, ok := byName[RoleUser]
	if !ok {
		t.Fatal("нет встроенной роли user")
	}
	for _, p := range []Permission{PermViewStatus, PermListDevices, PermRunSpeedtest} {
		if !user.Has(p) {
			t.Errorf("user должен иметь %q", p)
		}
	}
	for _, p := range []Permission{PermBanDevices, PermManageVPN, PermManageUsers} {
		if user.Has(p) {
			t.Errorf("user НЕ должен иметь %q", p)
		}
	}
}

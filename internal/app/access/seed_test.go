package access

import (
	"context"
	"errors"
	"testing"

	"github.com/tumour/openwrt-bot/internal/domain"
)

func TestSeed_Execute_FreshStore(t *testing.T) {
	store := newMemStore(nil, nil)
	uc := NewSeed(store.Users(), store.Roles())
	ctx := context.Background()

	if err := uc.Execute(ctx, SeedInput{AdminID: 680982436}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	admin, err := store.Users().Get(ctx, 680982436)
	if err != nil || !admin.IsActive() || admin.Role != domain.RoleAdmin {
		t.Errorf("env-админ = %+v (%v), want active admin", admin, err)
	}
	for _, name := range []domain.RoleName{domain.RoleAdmin, domain.RoleUser} {
		if _, err := store.Roles().Get(ctx, name); err != nil {
			t.Errorf("встроенная роль %q не создана: %v", name, err)
		}
	}
}

// «Есть — до свидания»: существующий юзер env-сидом не трогается, даже если
// он разжалован или pending. Env — не источник правды после первого старта.
func TestSeed_Execute_ExistingAdminUntouched(t *testing.T) {
	demoted := domain.NewActiveUser(680982436, "разжалован", domain.RoleUser)
	store := newMemStore([]domain.User{demoted}, domain.DefaultRoles())
	uc := NewSeed(store.Users(), store.Roles())
	ctx := context.Background()

	if err := uc.Execute(ctx, SeedInput{AdminID: 680982436}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := store.Users().Get(ctx, 680982436)
	if got.Role != domain.RoleUser {
		t.Errorf("роль = %q — seed не должен был трогать существующего", got.Role)
	}
}

// Политика догона: у admin-роли из «старого деплоя» нет новых прав каталога —
// seed дополняет. Кастомизация роли user при этом неприкосновенна.
func TestSeed_Execute_AdminRoleCatchUp(t *testing.T) {
	staleAdmin := domain.Role{Name: domain.RoleAdmin, Permissions: []domain.Permission{domain.PermViewStatus}}
	customUser := domain.Role{Name: domain.RoleUser, Permissions: []domain.Permission{domain.PermViewStatus}}
	store := newMemStore(nil, []domain.Role{staleAdmin, customUser})
	uc := NewSeed(store.Users(), store.Roles())
	ctx := context.Background()

	if err := uc.Execute(ctx, SeedInput{AdminID: 1}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	admin, _ := store.Roles().Get(ctx, domain.RoleAdmin)
	for _, p := range domain.AllPermissions() {
		if !admin.Has(p) {
			t.Errorf("admin после догона должен иметь %q", p)
		}
	}
	user, _ := store.Roles().Get(ctx, domain.RoleUser)
	if len(user.Permissions) != 1 || user.Permissions[0] != domain.PermViewStatus {
		t.Errorf("кастомизация роли user должна пережить seed, got %v", user.Permissions)
	}
}

func TestSeed_Execute_Idempotent(t *testing.T) {
	store := newMemStore(nil, nil)
	uc := NewSeed(store.Users(), store.Roles())
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if err := uc.Execute(ctx, SeedInput{AdminID: 1}); err != nil {
			t.Fatalf("прогон %d: %v", i, err)
		}
	}
	users, _ := store.Users().All(ctx)
	roles, _ := store.Roles().All(ctx)
	if len(users) != 1 || len(roles) != 2 {
		t.Errorf("после трёх прогонов: users=%d roles=%d, want 1/2", len(users), len(roles))
	}
}

func TestSeed_Execute_InvalidAdminID(t *testing.T) {
	store := newMemStore(nil, nil)
	uc := NewSeed(store.Users(), store.Roles())

	for _, id := range []int64{0, -1} {
		if err := uc.Execute(context.Background(), SeedInput{AdminID: id}); !errors.Is(err, domain.ErrInvalidUserID) {
			t.Errorf("id %d: err = %v, want ErrInvalidUserID (бот не должен стартовать без админа)", id, err)
		}
	}
}

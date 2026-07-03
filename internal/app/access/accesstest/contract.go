// Package accesstest — контрактный тест-сьют портов фичи access. Контракт
// принадлежит фиче (порту), реализация — адаптеру: любой движок хранилища
// (accessjson, завтра accesssqlite/accesspg) доказывает совместимость одним
// вызовом Run из своих тестов. Новый адаптер, прошедший сьют, можно вставлять
// в composition root без перепроверки use cases.
package accesstest

import (
	"context"
	"errors"
	"testing"

	"github.com/tumour/openwrt-bot/internal/app/access"
	"github.com/tumour/openwrt-bot/internal/domain"
)

// Factory отдаёт СВЕЖУЮ пару сторов (чистое состояние) на каждый вызов.
type Factory func(t *testing.T) (access.UserStore, access.RoleStore)

// Run прогоняет контракт обоих портов против реализации.
func Run(t *testing.T, newStore Factory) {
	t.Run("UserStore", func(t *testing.T) { runUsers(t, newStore) })
	t.Run("RoleStore", func(t *testing.T) { runRoles(t, newStore) })
}

func runUsers(t *testing.T, newStore Factory) {
	ctx := context.Background()

	t.Run("Get отсутствующего — ErrUserNotFound", func(t *testing.T) {
		users, _ := newStore(t)
		if _, err := users.Get(ctx, 42); !errors.Is(err, domain.ErrUserNotFound) {
			t.Errorf("err = %v, want ErrUserNotFound", err)
		}
	})

	t.Run("Put+Get: pending и active выживают со всеми полями", func(t *testing.T) {
		users, _ := newStore(t)
		pending := domain.NewPendingUser(1, "Батя")
		active := domain.NewActiveUser(2, "Мама", domain.RoleAdmin)
		for _, u := range []domain.User{pending, active} {
			if err := users.Put(ctx, u); err != nil {
				t.Fatalf("put %d: %v", u.ID, err)
			}
		}
		for _, want := range []domain.User{pending, active} {
			got, err := users.Get(ctx, want.ID)
			if err != nil {
				t.Fatalf("get %d: %v", want.ID, err)
			}
			if got != want {
				t.Errorf("get %d = %+v, want %+v", want.ID, got, want)
			}
		}
	})

	t.Run("Put — upsert: повторный Put заменяет", func(t *testing.T) {
		users, _ := newStore(t)
		u := domain.NewPendingUser(1, "до")
		if err := users.Put(ctx, u); err != nil {
			t.Fatal(err)
		}
		if err := u.Approve(domain.RoleUser); err != nil {
			t.Fatal(err)
		}
		if err := users.Put(ctx, u); err != nil {
			t.Fatal(err)
		}
		got, _ := users.Get(ctx, 1)
		if !got.IsActive() {
			t.Errorf("после upsert = %+v, want active", got)
		}
		all, _ := users.All(ctx)
		if len(all) != 1 {
			t.Errorf("после upsert %d записей, want 1", len(all))
		}
	})

	t.Run("All возвращает всех", func(t *testing.T) {
		users, _ := newStore(t)
		for id := int64(1); id <= 3; id++ {
			if err := users.Put(ctx, domain.NewPendingUser(domain.UserID(id), "")); err != nil {
				t.Fatal(err)
			}
		}
		all, err := users.All(ctx)
		if err != nil || len(all) != 3 {
			t.Errorf("all = %d записей (%v), want 3", len(all), err)
		}
	})

	t.Run("Delete удаляет; отсутствующего — ErrUserNotFound", func(t *testing.T) {
		users, _ := newStore(t)
		if err := users.Put(ctx, domain.NewPendingUser(1, "")); err != nil {
			t.Fatal(err)
		}
		if err := users.Delete(ctx, 1); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if _, err := users.Get(ctx, 1); !errors.Is(err, domain.ErrUserNotFound) {
			t.Error("после Delete юзер не должен находиться")
		}
		if err := users.Delete(ctx, 1); !errors.Is(err, domain.ErrUserNotFound) {
			t.Errorf("повторный delete: err = %v, want ErrUserNotFound", err)
		}
	})
}

func runRoles(t *testing.T, newStore Factory) {
	ctx := context.Background()

	t.Run("Get отсутствующей — ErrRoleNotFound", func(t *testing.T) {
		_, roles := newStore(t)
		if _, err := roles.Get(ctx, "ghost"); !errors.Is(err, domain.ErrRoleNotFound) {
			t.Errorf("err = %v, want ErrRoleNotFound", err)
		}
	})

	t.Run("Put+Get: права выживают в порядке и составе", func(t *testing.T) {
		_, roles := newStore(t)
		want := domain.Role{Name: "moderator", Permissions: []domain.Permission{
			domain.PermBanDevices, domain.PermViewStatus,
		}}
		if err := roles.Put(ctx, want); err != nil {
			t.Fatal(err)
		}
		got, err := roles.Get(ctx, "moderator")
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != want.Name || len(got.Permissions) != 2 ||
			!got.Has(domain.PermBanDevices) || !got.Has(domain.PermViewStatus) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("Put — upsert; All возвращает все", func(t *testing.T) {
		_, roles := newStore(t)
		if err := roles.Put(ctx, domain.Role{Name: "x"}); err != nil {
			t.Fatal(err)
		}
		if err := roles.Put(ctx, domain.Role{Name: "x", Permissions: []domain.Permission{domain.PermViewStatus}}); err != nil {
			t.Fatal(err)
		}
		if err := roles.Put(ctx, domain.Role{Name: "y"}); err != nil {
			t.Fatal(err)
		}
		all, err := roles.All(ctx)
		if err != nil || len(all) != 2 {
			t.Fatalf("all = %d ролей (%v), want 2", len(all), err)
		}
		got, _ := roles.Get(ctx, "x")
		if !got.Has(domain.PermViewStatus) {
			t.Error("upsert должен был заменить права роли x")
		}
	})

	t.Run("Delete удаляет; отсутствующей — ErrRoleNotFound", func(t *testing.T) {
		_, roles := newStore(t)
		if err := roles.Put(ctx, domain.Role{Name: "x"}); err != nil {
			t.Fatal(err)
		}
		if err := roles.Delete(ctx, "x"); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if err := roles.Delete(ctx, "x"); !errors.Is(err, domain.ErrRoleNotFound) {
			t.Errorf("повторный delete: err = %v, want ErrRoleNotFound", err)
		}
	})
}

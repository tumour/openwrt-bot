package accessjson

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/tumour/openwrt-bot/internal/adapter/secondary/system"
	"github.com/tumour/openwrt-bot/internal/app/access"
	"github.com/tumour/openwrt-bot/internal/app/access/accesstest"
	"github.com/tumour/openwrt-bot/internal/domain"
)

func newTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "json")
	s := New(system.NewOSFileReader(), system.NewOSFileWriter(), dir)
	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("init: %v", err)
	}
	return s, dir
}

// Контракт портов: адаптер обязан вести себя неотличимо от любого другого
// хранилища фичи access. Будущий accesssqlite прогонит ровно этот же сьют.
func TestStore_Contract(t *testing.T) {
	accesstest.Run(t, func(t *testing.T) (access.UserStore, access.RoleStore) {
		s, _ := newTestStore(t)
		return s.Users(), s.Roles()
	})
}

// JSON-специфика: состояние живёт в файлах, а не в памяти Store —
// новый инстанс над тем же каталогом видит те же данные (рестарт бота).
func TestStore_PersistsAcrossReopen(t *testing.T) {
	s1, dir := newTestStore(t)
	ctx := context.Background()

	if err := s1.Users().Put(ctx, domain.NewActiveUser(1, "admin", domain.RoleAdmin)); err != nil {
		t.Fatal(err)
	}
	if err := s1.Roles().Put(ctx, domain.Role{Name: domain.RoleAdmin, Permissions: domain.AllPermissions()}); err != nil {
		t.Fatal(err)
	}

	s2 := New(system.NewOSFileReader(), system.NewOSFileWriter(), dir)
	u, err := s2.Users().Get(ctx, 1)
	if err != nil || u.Role != domain.RoleAdmin {
		t.Errorf("после reopen: %+v, %v", u, err)
	}
	r, err := s2.Roles().Get(ctx, domain.RoleAdmin)
	if err != nil || !r.Has(domain.PermManageUsers) {
		t.Errorf("после reopen: %+v, %v", r, err)
	}
}

// Битый файл — явная ошибка, а не молчаливо пустой список (иначе бот
// «забыл бы» всех пользователей и пересоздал админа из env).
func TestStore_CorruptFile_Fails(t *testing.T) {
	s, dir := newTestStore(t)
	if err := os.WriteFile(filepath.Join(dir, "users.json"), []byte("{битый"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Users().All(context.Background()); err == nil {
		t.Error("битый users.json должен давать ошибку")
	}
}

// Неизвестное право в roles.json пропускается (сжатие каталога — штатный
// случай), а не валит загрузку.
func TestStore_UnknownPermissionSkipped(t *testing.T) {
	s, dir := newTestStore(t)
	raw := `{"v":1,"items":[{"name":"admin","permissions":["manage_users","fly_to_moon"]}]}`
	if err := os.WriteFile(filepath.Join(dir, "roles.json"), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := s.Roles().Get(context.Background(), domain.RoleAdmin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.Has(domain.PermManageUsers) || len(r.Permissions) != 1 {
		t.Errorf("права = %v, want только manage_users", r.Permissions)
	}
}

// Смоук конкуренции под -race: telebot-handlers зовут порты параллельно.
func TestStore_ConcurrentAccess(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 1; i <= 8; i++ {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			_ = s.Users().Put(ctx, domain.NewPendingUser(domain.UserID(id), "x"))
			_, _ = s.Users().All(ctx)
			_ = s.Roles().Put(ctx, domain.Role{Name: domain.RoleUser})
			_, _ = s.Roles().All(ctx)
		}(int64(i))
	}
	wg.Wait()
	all, err := s.Users().All(ctx)
	if err != nil || len(all) != 8 {
		t.Errorf("после конкурентных Put: %d юзеров (%v), want 8", len(all), err)
	}
}

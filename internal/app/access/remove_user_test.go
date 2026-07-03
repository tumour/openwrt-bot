package access

import (
	"context"
	"errors"
	"testing"

	"github.com/tumour/openwrt-bot/internal/domain"
)

func TestRemoveUser_Execute_OK(t *testing.T) {
	store := defaultRolesStore(
		domain.NewActiveUser(2, "гость", domain.RoleUser),
		domain.NewPendingUser(3, "заявка"),
	)
	uc := NewRemoveUser(store.Users(), store.Roles())
	ctx := context.Background()

	for _, target := range []domain.UserID{2, 3} { // активный и pending
		if _, err := uc.Execute(ctx, RemoveUserInput{ActorID: 1, TargetID: target}); err != nil {
			t.Errorf("удаление %d: %v", target, err)
		}
		if _, err := store.Users().Get(ctx, target); !errors.Is(err, domain.ErrUserNotFound) {
			t.Errorf("юзер %d должен быть удалён", target)
		}
	}
}

func TestRemoveUser_Execute_LastAdminGuard(t *testing.T) {
	store := defaultRolesStore()
	uc := NewRemoveUser(store.Users(), store.Roles())
	ctx := context.Background()

	// Единственный админ удаляет сам себя — бот остался бы неуправляемым.
	if _, err := uc.Execute(ctx, RemoveUserInput{ActorID: 1, TargetID: 1}); !errors.Is(err, domain.ErrLastAdmin) {
		t.Errorf("err = %v, want ErrLastAdmin", err)
	}

	// Со вторым админом самоудаление легально.
	if err := store.Users().Put(ctx, domain.NewActiveUser(5, "второй", domain.RoleAdmin)); err != nil {
		t.Fatal(err)
	}
	if _, err := uc.Execute(ctx, RemoveUserInput{ActorID: 1, TargetID: 1}); err != nil {
		t.Errorf("с двумя админами самоудаление должно пройти: %v", err)
	}
}

func TestRemoveUser_Execute_Forbidden(t *testing.T) {
	store := defaultRolesStore(domain.NewActiveUser(2, "гость", domain.RoleUser))
	uc := NewRemoveUser(store.Users(), store.Roles())

	if _, err := uc.Execute(context.Background(), RemoveUserInput{ActorID: 2, TargetID: 1}); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("err = %v, want ErrForbidden", err)
	}
}

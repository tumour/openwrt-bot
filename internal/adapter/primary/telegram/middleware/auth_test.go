package middleware

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/tumour/openwrt-bot/internal/app/access"
	"github.com/tumour/openwrt-bot/internal/domain"
	tele "gopkg.in/telebot.v3"
)

// fakeCtx — минимальный мок tele.Context: интерфейс огромный, поэтому embed'им
// его (nil) и переопределяем только то, что трогают Auth и Put/Get контекста.
// Вызов любого другого метода уронит тест паникой — это желаемое поведение.
type fakeCtx struct {
	tele.Context
	sender   *tele.User
	text     string
	callback *tele.Callback
	store    map[string]any
}

func newFakeCtx(sender *tele.User, text string) *fakeCtx {
	return &fakeCtx{sender: sender, text: text, store: map[string]any{}}
}

func (f *fakeCtx) Sender() *tele.User       { return f.sender }
func (f *fakeCtx) Text() string             { return f.text }
func (f *fakeCtx) Callback() *tele.Callback { return f.callback }
func (f *fakeCtx) Set(k string, v any)      { f.store[k] = v }
func (f *fakeCtx) Get(k string) any         { return f.store[k] }

// fakeChecker — управляемый AccessChecker.
type fakeChecker struct {
	out access.CheckOutput
	err error
}

func (f fakeChecker) Execute(context.Context, access.CheckInput) (access.CheckOutput, error) {
	return f.out, f.err
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// nextSpy возвращает HandlerFunc-заглушку и флаг "вызвана ли она".
func nextSpy() (tele.HandlerFunc, *bool) {
	called := false
	return func(tele.Context) error {
		called = true
		return nil
	}, &called
}

func allowedChecker() fakeChecker {
	return fakeChecker{out: access.CheckOutput{
		Allowed: true,
		Grant: access.Grant{
			User: domain.NewActiveUser(42, "admin", domain.RoleAdmin),
			Role: domain.Role{Name: domain.RoleAdmin, Permissions: domain.AllPermissions()},
		},
	}}
}

func TestAuth_AllowedUser_PassesWithGrant(t *testing.T) {
	next, called := nextSpy()
	mw := Auth(allowedChecker(), nil, discardLogger())
	c := newFakeCtx(&tele.User{ID: 42}, "/status")

	if err := mw(next)(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !*called {
		t.Error("next должен быть вызван для допущенного")
	}
	g, ok := GrantFrom(c)
	if !ok || !g.Has(domain.PermManageUsers) {
		t.Errorf("Grant должен лежать в контексте, got %+v ok=%v", g, ok)
	}
}

func TestAuth_UnknownUser_SilentlyDropped(t *testing.T) {
	next, called := nextSpy()
	onRequest, requested := nextSpy()
	mw := Auth(fakeChecker{}, onRequest, discardLogger())

	if err := mw(next)(newFakeCtx(&tele.User{ID: 999}, "/status")); err != nil {
		t.Fatalf("чужой юзер должен игнорироваться молча, got: %v", err)
	}
	if *called {
		t.Error("next НЕ должен вызываться для недопущенного")
	}
	if *requested {
		t.Error("не-/start от незнакомца не должен открывать approve-flow")
	}
}

// /start незнакомца — единственная дверь в approve-flow.
func TestAuth_UnknownUser_StartOpensRequestFlow(t *testing.T) {
	next, called := nextSpy()
	onRequest, requested := nextSpy()
	mw := Auth(fakeChecker{}, onRequest, discardLogger())

	if err := mw(next)(newFakeCtx(&tele.User{ID: 999}, "/start")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !*requested {
		t.Error("/start незнакомца должен уходить в approve-flow")
	}
	if *called {
		t.Error("next НЕ должен вызываться для недопущенного")
	}
}

// Callback с данными «/start» — не сообщение: approve-flow не открывается.
func TestAuth_UnknownUser_CallbackIsNotStart(t *testing.T) {
	next, called := nextSpy()
	onRequest, requested := nextSpy()
	mw := Auth(fakeChecker{}, onRequest, discardLogger())

	c := newFakeCtx(&tele.User{ID: 999}, "/start")
	c.callback = &tele.Callback{}
	if err := mw(next)(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *requested || *called {
		t.Error("callback незнакомца должен молча отбрасываться")
	}
}

func TestAuth_NilSender_SilentlyDropped(t *testing.T) {
	// У части апдейтов (channel post) отправителя нет — не должны паниковать.
	next, called := nextSpy()
	mw := Auth(allowedChecker(), nil, discardLogger())

	if err := mw(next)(newFakeCtx(nil, "x")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *called {
		t.Error("next НЕ должен вызываться без отправителя")
	}
}

// Ошибка хранилища = отказ в доступе (fail closed), а не пропуск.
func TestAuth_CheckerError_FailsClosed(t *testing.T) {
	next, called := nextSpy()
	mw := Auth(fakeChecker{err: errors.New("db down")}, nil, discardLogger())

	if err := mw(next)(newFakeCtx(&tele.User{ID: 42}, "/status")); err != nil {
		t.Fatalf("ошибка чекера гасится (уже залогирована), got: %v", err)
	}
	if *called {
		t.Error("при ошибке проверки next вызываться НЕ должен")
	}
}

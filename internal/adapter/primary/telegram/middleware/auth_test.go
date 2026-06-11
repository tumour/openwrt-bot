package middleware

import (
	"io"
	"log/slog"
	"testing"

	tele "gopkg.in/telebot.v3"
)

// fakeCtx — минимальный мок tele.Context: интерфейс огромный, поэтому embed'им
// его (nil) и переопределяем только то, что трогает Auth. Вызов любого другого
// метода уронит тест паникой — это желаемое поведение (Auth не должен лезть дальше).
type fakeCtx struct {
	tele.Context
	sender *tele.User
}

func (f *fakeCtx) Sender() *tele.User { return f.sender }
func (f *fakeCtx) Text() string       { return "/status" }

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

func TestAuth_AllowedUser_Passes(t *testing.T) {
	next, called := nextSpy()
	mw := Auth([]int64{42}, discardLogger())

	err := mw(next)(&fakeCtx{sender: &tele.User{ID: 42}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !*called {
		t.Error("next должен быть вызван для whitelist-юзера")
	}
}

func TestAuth_UnknownUser_SilentlyDropped(t *testing.T) {
	next, called := nextSpy()
	mw := Auth([]int64{42}, discardLogger())

	err := mw(next)(&fakeCtx{sender: &tele.User{ID: 999}})
	if err != nil {
		t.Fatalf("чужой юзер должен игнорироваться молча, got: %v", err)
	}
	if *called {
		t.Error("next НЕ должен вызываться для юзера вне whitelist")
	}
}

func TestAuth_NilSender_SilentlyDropped(t *testing.T) {
	// У части апдейтов (channel post) отправителя нет — не должны паниковать.
	next, called := nextSpy()
	mw := Auth([]int64{42}, discardLogger())

	err := mw(next)(&fakeCtx{sender: nil})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *called {
		t.Error("next НЕ должен вызываться без отправителя")
	}
}

func TestAuth_EmptyWhitelist_DropsEveryone(t *testing.T) {
	next, called := nextSpy()
	mw := Auth(nil, discardLogger())

	if err := mw(next)(&fakeCtx{sender: &tele.User{ID: 42}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *called {
		t.Error("пустой whitelist = никто не авторизован")
	}
}

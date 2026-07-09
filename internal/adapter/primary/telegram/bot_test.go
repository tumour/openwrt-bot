package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// newTestBot — Bot поверх httptest-сервера. Handlers{} безопасен: method values
// на nil-указателях создаются без вызова (см. router_test.go), а пустой
// whitelist глушит все апдейты в Auth. URL подменяется после NewBot — поле
// telebot публичное, в продовый API ничего не течёт (как и connectBackoff).
func newTestBot(t *testing.T, apiURL string) *Bot {
	t.Helper()
	b, err := NewBot(Config{Token: "test"}, testLogger(), Handlers{})
	if err != nil {
		t.Fatalf("NewBot: %v", err)
	}
	b.bot.URL = apiURL
	b.connectBackoff = backoff{min: time.Millisecond, max: 4 * time.Millisecond}
	return b
}

const getMeOK = `{"ok":true,"result":{"id":1,"is_bot":true,"username":"testbot"}}`

func TestConnect_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, getMeOK)
	}))
	defer srv.Close()

	b := newTestBot(t, srv.URL)
	if b.Connected() {
		t.Error("Connected() = true до подключения, want false")
	}
	if !b.connect(context.Background()) {
		t.Fatal("connect() = false, want true")
	}
	if b.bot.Me == nil || b.bot.Me.Username != "testbot" {
		t.Errorf("bot.Me = %+v, want username testbot", b.bot.Me)
	}
	if !b.Connected() {
		t.Error("Connected() = false после подключения, want true")
	}
}

func TestConnect_RetriesUntilSuccess(t *testing.T) {
	var served atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if served.Add(1) <= 2 {
			fmt.Fprint(w, `{"ok":false,"error_code":400,"description":"boom"}`)
			return
		}
		fmt.Fprint(w, getMeOK)
	}))
	defer srv.Close()

	b := newTestBot(t, srv.URL)
	if !b.connect(context.Background()) {
		t.Fatal("connect() = false, want true после дозвона")
	}
	if n := served.Load(); n < 3 {
		t.Errorf("запросов = %d, want ≥3 (2 ошибки + успех)", n)
	}
}

// Регрессия: отмена ctx обязана прерывать backoff-сон.
func TestConnect_CtxCancelDuringSleep(t *testing.T) {
	requested := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case requested <- struct{}{}:
		default:
		}
		fmt.Fprint(w, `{"ok":false,"error_code":400,"description":"boom"}`)
	}))
	defer srv.Close()

	b := newTestBot(t, srv.URL)
	b.connectBackoff = backoff{min: 10 * time.Second, max: 10 * time.Second} // сон дольше теста

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan bool, 1)
	go func() { done <- b.connect(ctx) }()

	<-requested // первая ошибка получена, connect ушёл в сон
	cancel()
	select {
	case ok := <-done:
		if ok {
			t.Error("connect() = true после отмены ctx, want false")
		}
	case <-time.After(time.Second):
		t.Fatal("connect спит вместо выхода: select на ctx.Done() во сне сломан")
	}
}

// Решение итерации 10: битый токен (401) НЕ прекращает retry — процесс живёт,
// диагноз в логе. Тест доказывает ≥2 попыток на постоянном 401.
func TestConnect_AuthErrorKeepsRetrying(t *testing.T) {
	var served atomic.Int32
	twoServed := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if served.Add(1) == 2 {
			close(twoServed)
		}
		fmt.Fprint(w, `{"ok":false,"error_code":401,"description":"Unauthorized"}`)
	}))
	defer srv.Close()

	b := newTestBot(t, srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan bool, 1)
	go func() { done <- b.connect(ctx) }()

	select {
	case <-twoServed: // вторая попытка состоялась — на 401 не сдались
	case <-time.After(2 * time.Second):
		t.Fatal("второй попытки после 401 не было — connect сдался на битом токене")
	}
	cancel()
	if ok := <-done; ok {
		t.Error("connect() = true, want false (ctx отменён)")
	}
}

// Регрессия Stop-ловушки telebot: отмена ctx ДО дозвона обязана возвращать
// Run без зависания (Stop() без запущенного Start() виснет навсегда).
func TestRun_CancelBeforeConnected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"ok":false,"error_code":400,"description":"boom"}`)
	}))
	defer srv.Close()

	b := newTestBot(t, srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // отменяем сразу — Telegram так и не станет доступен

	done := make(chan error, 1)
	go func() { done <- b.Run(ctx) }()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run() = %v, want nil", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run завис: похоже, Stop() позвали без запущенного Start()")
	}
}

// Анонс при старте: после connect бот шлёт каждому из whitelist сообщение с
// постоянной клавиатурой — свежая раскладка приезжает сама, без ручного /start
// после деплоя. Ошибка отправки одному юзеру не мешает остальным.
func TestRun_AnnouncesKeyboardToWhitelist(t *testing.T) {
	var mu sync.Mutex
	var gotChats []string
	var gotMarkup []bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bottest/getMe":
			fmt.Fprint(w, getMeOK)
		case "/bottest/setMyCommands":
			fmt.Fprint(w, `{"ok":true,"result":true}`)
		case "/bottest/sendMessage":
			// telebot шлёт параметры JSON-телом; reply_markup внутри — строка с JSON.
			var params struct {
				ChatID      string `json:"chat_id"`
				ReplyMarkup string `json:"reply_markup"`
			}
			_ = json.NewDecoder(r.Body).Decode(&params)
			var markup struct {
				Keyboard [][]struct{ Text string } `json:"keyboard"`
			}
			hasKeyboard := json.Unmarshal([]byte(params.ReplyMarkup), &markup) == nil &&
				len(markup.Keyboard) > 0
			mu.Lock()
			gotChats = append(gotChats, params.ChatID)
			gotMarkup = append(gotMarkup, hasKeyboard)
			mu.Unlock()
			// Первому юзеру — ошибка (не начинал чат): не должна помешать второму.
			if params.ChatID == "42" {
				fmt.Fprint(w, `{"ok":false,"error_code":403,"description":"Forbidden: bot was blocked"}`)
				return
			}
			fmt.Fprint(w, `{"ok":true,"result":{"message_id":1,"chat":{"id":1}}}`)
		case "/bottest/getUpdates":
			time.Sleep(10 * time.Millisecond)
			fmt.Fprint(w, `{"ok":true,"result":[]}`)
		default:
			fmt.Fprint(w, `{"ok":true,"result":true}`)
		}
	}))
	defer srv.Close()

	b, err := NewBot(Config{Token: "test", AllowedUserIDs: []int64{42, 43}}, testLogger(), Handlers{})
	if err != nil {
		t.Fatalf("NewBot: %v", err)
	}
	b.bot.URL = srv.URL
	b.connectBackoff = backoff{min: time.Millisecond, max: 4 * time.Millisecond}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- b.Run(ctx) }()

	deadline := time.After(2 * time.Second)
	for {
		mu.Lock()
		n := len(gotChats)
		mu.Unlock()
		if n >= 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("анонс не дошёл до обоих юзеров из whitelist")
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	if err := <-done; err != nil {
		t.Errorf("Run() = %v, want nil", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if gotChats[0] != "42" || gotChats[1] != "43" {
		t.Errorf("анонс ушёл чатам %v, want [42 43]", gotChats)
	}
	for i, ok := range gotMarkup {
		if !ok {
			t.Errorf("анонс #%d без reply-клавиатуры", i)
		}
	}
}

// Сквозная проверка happy path: connect → меню → поллинг → graceful shutdown
// (паритет итерации 8: Stop отменяет long-poll, Run возвращает nil).
func TestRun_HappyPathAndGracefulShutdown(t *testing.T) {
	var gotGetMe, gotSetCommands, gotGetUpdates atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bottest/getMe":
			gotGetMe.Store(true)
			fmt.Fprint(w, getMeOK)
		case "/bottest/setMyCommands":
			gotSetCommands.Store(true)
			fmt.Fprint(w, `{"ok":true,"result":true}`)
		case "/bottest/getUpdates":
			gotGetUpdates.Store(true)
			time.Sleep(20 * time.Millisecond) // имитация long-poll
			fmt.Fprint(w, `{"ok":true,"result":[]}`)
		default:
			t.Errorf("неожиданный вызов API: %s", r.URL.Path)
			fmt.Fprint(w, `{"ok":true,"result":true}`)
		}
	}))
	defer srv.Close()

	b := newTestBot(t, srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- b.Run(ctx) }()

	// Дать пройти connect+меню и хотя бы одному getUpdates.
	deadline := time.After(2 * time.Second)
	for !gotGetUpdates.Load() {
		select {
		case <-deadline:
			t.Fatal("getUpdates так и не случился")
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run() = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run не завершился после отмены ctx")
	}
	if !gotGetMe.Load() || !gotSetCommands.Load() {
		t.Errorf("фазы не отработали: getMe=%v setMyCommands=%v", gotGetMe.Load(), gotSetCommands.Load())
	}
}

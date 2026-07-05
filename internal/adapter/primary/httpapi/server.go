// Package httpapi — второй driving adapter: локальный HTTP API поверх тех же
// use cases, что и Telegram. Потребитель — rpcd ucode-плагин LuCI-панели,
// ходит с localhost. Имя пакета httpapi, а не http — чтобы не шадоуить stdlib.
//
// Контракты API:
//   - успех — всегда JSON: мутации отвечают 200 {"ok":true}, ошибки —
//     {"error":"..."} (клиенту-плагину удобен инвариант «каждый ответ — объект»);
//   - error boundary как у telegram Log-middleware: клиенту generic-сообщение
//     без внутренностей exec, полная цепочка — в slog;
//   - 404/405 — plain-text от stdlib ServeMux (осознанно: JSON-404 требует
//     catch-all, а 405 у stdlib не перехватывается; машинный контракт для
//     единственного доверенного клиента — статус-код, не тело).
//
// Security-модель: аутентификации нет, API задуман только для loopback
// (браузерный CSRF до loopback роутера не дотягивается, клиент — свой ucode);
// не-loopback адрес не запрещаем — env в зоне ответственности владельца,
// но Run предупреждает Warn.
package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// Deps — зависимости endpoint'ов, по образцу telegram.Handlers: конкретные
// типы use cases в struct дают compile-time полноту проводки composition root
// (добавил поле — забыл прокинуть = nil deref на первом запросе, видно сразу).
type Deps struct {
	// TelegramUp — состояние Telegram-канала для /health. func() bool, а не
	// импорт telegram: primary-адаптеры друг о друге не знают, связывает main.
	TelegramUp func() bool
	Logger     *slog.Logger
}

// Server — HTTP API с lifecycle-контрактом telegram.Bot.Run: блокирующий
// Run(ctx) до отмены ctx, штатная отмена → nil.
type Server struct {
	addr   string
	deps   Deps
	logger *slog.Logger
	srv    *http.Server
}

const (
	// requestTimeout — потолок на обработку одного запроса (exec'и nft/ubus);
	// паритет handlerTimeout telegram-адаптера.
	requestTimeout = 10 * time.Second
	// shutdownTimeout — потолок ожидания in-flight запросов на выходе;
	// паритет waitHandlers.
	shutdownTimeout = 5 * time.Second

	healthPath = "/api/v1/health"
)

func NewServer(addr string, deps Deps) *Server {
	s := &Server{addr: addr, deps: deps, logger: deps.Logger}
	s.srv = &http.Server{
		Handler:           s.accessLog(s.routes()),
		ReadHeaderTimeout: 5 * time.Second,  // slowloris
		ReadTimeout:       10 * time.Second, // заголовки + тело целиком
		WriteTimeout:      15 * time.Second, // > requestTimeout: долгий use case успевает ответить
		IdleTimeout:       60 * time.Second, // keep-alive для rpcd
		// Паники и протокольные ошибки net/http — в structured log.
		ErrorLog: slog.NewLogLogger(deps.Logger.Handler(), slog.LevelError),
	}
	return s
}

func (s *Server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+healthPath, s.handleHealth)
	return mux
}

// Run блокирующе работает до отмены ctx. Listen — синхронно: занятый или
// битый HTTP_ADDR — класс config-ошибок, по ним валимся сразу (fail fast,
// паритет config.Load), а не тихо живём без API.
func (s *Server) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("http api: listen %s: %w", s.addr, err)
	}
	return s.serve(ctx, ln)
}

// serve отделён от Run ради lifecycle-тестов: тест слушает 127.0.0.1:0
// и берёт фактический адрес из ln.Addr().
func (s *Server) serve(ctx context.Context, ln net.Listener) error {
	if addr, ok := ln.Addr().(*net.TCPAddr); ok && !addr.IP.IsLoopback() {
		s.logger.Warn("http api слушает не-loopback адрес, а аутентификации у него нет", "addr", ln.Addr())
	}
	// Паритет track-middleware телеграма: ctx каждого запроса наследует
	// run-ctx — SIGTERM отменяет in-flight exec'и разом (итерация 8).
	s.srv.BaseContext = func(net.Listener) context.Context { return ctx }

	s.logger.Info("http api started", "addr", ln.Addr().String())
	errCh := make(chan error, 1)
	go func() { errCh <- s.srv.Serve(ln) }()

	select {
	case <-ctx.Done():
		s.logger.Info("shutting down http api")
		shCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := s.srv.Shutdown(shCtx); err != nil {
			s.logger.Warn("не все запросы завершились до таймаута, рву соединения", "err", err)
			_ = s.srv.Close()
		}
		<-errCh // Serve уже вернул ErrServerClosed; дренируем, чтобы не течь горутиной
		return nil
	case err := <-errCh:
		// Accept умер сам — честный выход, procd перезапустит процесс.
		return fmt.Errorf("http api: %w", err)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	tg := "connecting"
	if s.deps.TelegramUp() {
		tg = "connected"
	}
	s.writeJSON(w, http.StatusOK, healthResponse{Status: "ok", Telegram: tg})
}

type (
	// healthResponse: Status всегда "ok" — сам факт ответа означает, что API
	// жив; Telegram — состояние канала ("connected"/"connecting" — при вечном
	// оффлайне, например с битым токеном, честно остаётся "connecting").
	healthResponse struct {
		Status   string `json:"status"`
		Telegram string `json:"telegram"`
	}
	errorResponse struct {
		Error string `json:"error"`
	}
	okResponse struct {
		OK bool `json:"ok"`
	}
)

func (s *Server) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Заголовки уже ушли — клиенту не помочь, фиксируем для диагностики.
		s.logger.Warn("http api: не удалось записать ответ", "err", err)
	}
}

func (s *Server) writeError(w http.ResponseWriter, status int, msg string) {
	s.writeJSON(w, status, errorResponse{Error: msg})
}

// internalError — error boundary (контракт telegram Log-middleware): клиенту
// generic-сообщение без внутренностей exec/stderr, полная цепочка — в slog.
func (s *Server) internalError(w http.ResponseWriter, r *http.Request, err error) {
	s.logger.Error("http api: запрос упал", "method", r.Method, "path", r.URL.Path, "err", err)
	s.writeError(w, http.StatusInternalServerError, "внутренняя ошибка")
}

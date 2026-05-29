package logger

import (
	"log/slog"
	"os"
	"strings"
)

// New собирает slog.Logger по строковому уровню. slog — stdlib (Go 1.21+),
// без сторонних зависимостей. Текстовый handler для читаемости в `journalctl`/`logread`.
func New(level string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

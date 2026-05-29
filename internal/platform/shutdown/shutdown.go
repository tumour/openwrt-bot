// Package shutdown — обёртка над signal.NotifyContext для graceful shutdown.
// Возвращает context, отменяемый при SIGINT/SIGTERM. Main передаёт его всем
// долгоживущим компонентам; они слушают <-ctx.Done() для остановки.
package shutdown

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// Context создаёт корневой контекст, который отменяется при SIGINT или SIGTERM.
// Возвращает context и cancel — cancel ОБЯЗАТЕЛЬНО вызвать через defer в main
// (даже при «graceful» exit), иначе утечка signal-handler'а.
func Context() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

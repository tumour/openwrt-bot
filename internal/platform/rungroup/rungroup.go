// Package rungroup — параллельный запуск долгоживущих компонентов с общей
// судьбой: первая ошибка отменяет контекст остальных, Wait ждёт всех и
// возвращает её. Семантика golang.org/x/sync/errgroup.WithContext, но на
// stdlib — репо сознательно не тянет зависимость ради тридцати строк.
package rungroup

import (
	"context"
	"sync"
)

// Group управляет горутинами с общей судьбой. Zero value непригоден —
// создавать только через New.
type Group struct {
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	errOnce sync.Once
	err     error
}

// New возвращает группу и производный контекст: его отменяет первая ошибка
// из Go-функций (и, для гигиены ресурсов, завершение Wait).
func New(ctx context.Context) (*Group, context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	return &Group{cancel: cancel}, ctx
}

// Go запускает fn в горутине. nil-результат судьбу группы не меняет:
// у долгоживущих компонентов nil означает штатное завершение по уже
// отменённому контексту, гасить остальных незачем.
func (g *Group) Go(fn func() error) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		if err := fn(); err != nil {
			g.errOnce.Do(func() {
				g.err = err
				g.cancel()
			})
		}
	}()
}

// Wait блокирует до завершения всех fn и возвращает первую ошибку или nil.
func (g *Group) Wait() error {
	g.wg.Wait()
	g.cancel()
	return g.err
}

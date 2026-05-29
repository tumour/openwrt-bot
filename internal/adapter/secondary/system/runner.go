package system

import (
	"context"
	"fmt"
	"os/exec"
)

// Runner — единственный интерфейс, через который весь secondary-уровень
// взаимодействует с внешними командами OS. В тестах adapter'ов (nftables, ubus)
// подкладывается фейк-Runner, и тесты проходят без реальных бинарей.
//
// Это применение dependency inversion: вся "infrastructure boundary" сужена
// до одной точки — мокая её, мокаешь весь exec.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// ExecRunner — продакшен-реализация через os/exec. Возвращает stdout как []byte.
// stderr на error попадает в Error() сообщении wrapped error'а — это упрощает
// диагностику без отдельного аргумента.
type ExecRunner struct{}

func NewExecRunner() *ExecRunner { return &ExecRunner{} }

func (r *ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	if err != nil {
		// exec.ExitError содержит Stderr — добавим его в сообщение для трассировки.
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("%s %v: %w (stderr: %s)", name, args, err, ee.Stderr)
		}
		return nil, fmt.Errorf("%s %v: %w", name, args, err)
	}
	return out, nil
}

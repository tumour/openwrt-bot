package system

import (
	"context"
	"errors"
	"fmt"
	"os"
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

// ExecError — ошибка внешней команды со структурными полями. Stderr отделён
// от текста сообщения сознательно: адаптеры типизируют ошибки по нему (nft
// пишет причину отказа в stderr), а матчить подстроку по всему Error() нельзя —
// туда входят имя команды и аргументы, что даёт ложные срабатывания.
type ExecError struct {
	Name   string
	Args   []string
	Stderr []byte // пуст, если процесс не запустился (ErrNotFound и т.п.)
	Err    error  // исходная ошибка os/exec
}

func (e *ExecError) Error() string {
	if len(e.Stderr) > 0 {
		return fmt.Sprintf("%s %v: %v (stderr: %s)", e.Name, e.Args, e.Err, e.Stderr)
	}
	return fmt.Sprintf("%s %v: %v", e.Name, e.Args, e.Err)
}

// Unwrap отдаёт исходную ошибку — errors.Is(err, exec.ErrNotFound) работает
// сквозь ExecError (на этом стоит librespeed-адаптер).
func (e *ExecError) Unwrap() error { return e.Err }

// ExecRunner — продакшен-реализация через os/exec. Возвращает stdout как []byte,
// на ошибке — *ExecError со stderr в отдельном поле.
type ExecRunner struct{}

func NewExecRunner() *ExecRunner { return &ExecRunner{} }

func (r *ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	// LC_ALL=C — сообщения утилит стабильно на английском: типизация ошибок
	// по stderr (nftables) не должна зависеть от локали системы.
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.Output()
	if err != nil {
		ee := &ExecError{Name: name, Args: args, Err: err}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			ee.Stderr = exitErr.Stderr
		}
		return nil, ee
	}
	return out, nil
}

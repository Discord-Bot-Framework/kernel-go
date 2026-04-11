package modules

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

type Evaluator struct {
	logger *slog.Logger
	i      *interp.Interpreter
	token  string

	mu sync.Mutex
}

func NewEvaluator(logger *slog.Logger, token string) *Evaluator {
	i := interp.New(interp.Options{
		GoPath: "",
	})
	_ = i.Use(stdlib.Symbols)

	return &Evaluator{logger: logger, token: token, i: i}
}

var (
	errEmptyCode        = errors.New("empty code")
	errDisallowedImport = errors.New("disallowed imports")
	errEvalFailed       = errors.New("eval failed")
)

func (e *Evaluator) Eval(ctx context.Context, code string) (string, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", errEmptyCode
	}

	if strings.Contains(code, "os/exec") || strings.Contains(code, "syscall") {
		return "", errDisallowedImport
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	deadline, ok := ctx.Deadline()
	if ok {
		if time.Until(deadline) <= 0 {
			return "", context.DeadlineExceeded
		}
	}

	v, err := e.i.Eval(code)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errEvalFailed, err)
	}

	if !v.IsValid() {
		return "", nil
	}

	if v.Kind() == reflect.Invalid {
		return "", nil
	}

	if v.CanInterface() {
		return fmt.Sprintf("%#v", v.Interface()), nil
	}

	return v.String(), nil
}

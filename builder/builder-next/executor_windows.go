package buildkit

import (
	"context"
	"errors"

	"github.com/moby/buildkit/executor"
)

func newExecutor(_ Opt) (executor.Executor, error) {
	return &winExecutor{}, nil
}

type winExecutor struct {
}

func (w *winExecutor) Run(ctx context.Context, id string, root executor.Mount, mounts []executor.Mount, process executor.ProcessInfo, started chan<- struct{}) (err error) {
	return errors.New("buildkit executor not implemented for windows")
}

func (w *winExecutor) Exec(ctx context.Context, id string, process executor.ProcessInfo) error {
	return errors.New("buildkit executor not implemented for windows")
}

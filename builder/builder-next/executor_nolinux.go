//go:build !linux

package buildkit

import (
	"context"
	"errors"
	"runtime"

	"github.com/moby/buildkit/executor"
	resourcetypes "github.com/moby/buildkit/executor/resources/types"
)

func newExecutor(Opt) (executor.Executor, error) {
	return &stubExecutor{}, nil
}

type stubExecutor struct{}

func (w *stubExecutor) Run(ctx context.Context, id string, root executor.Mount, mounts []executor.Mount, process executor.ProcessInfo, started chan<- struct{}) (resourcetypes.Recorder, error) {
	return nil, errors.New("buildkit executor not implemented for "+runtime.GOOS)
}

func (w *stubExecutor) Exec(ctx context.Context, id string, process executor.ProcessInfo) error {
	return errors.New("buildkit executor not implemented for "+runtime.GOOS)
}

//go:build !linux

package buildkit

import (
	"context"
	"errors"
	"runtime"

	"github.com/moby/buildkit/executor"
	resourcetypes "github.com/moby/buildkit/executor/resources/types"
	"github.com/moby/buildkit/util/network"
)

type stubExecutor struct{}

func (w *stubExecutor) Run(ctx context.Context, id string, root executor.Mount, mounts []executor.Mount, process executor.ProcessInfo, started chan<- struct{}) (resourcetypes.Recorder, error) {
	return nil, errors.New("buildkit executor not implemented for " + runtime.GOOS)
}

func (w *stubExecutor) Exec(ctx context.Context, id string, process executor.ProcessInfo) error {
	return errors.New("buildkit executor not implemented for " + runtime.GOOS)
}

// function stub created for GraphDriver
func newExecutorGD(executorOpts) (executor.Executor, network.ProxyProvider, error) {
	return &stubExecutor{}, nil, nil
}

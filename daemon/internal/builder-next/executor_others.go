//go:build !linux && !windows

package buildkit

import (
	"github.com/moby/buildkit/executor"
)

func newExecutor(executorOpts) (executor.Executor, error) {
	return &stubExecutor{}, nil
}

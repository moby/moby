package buildkit

import (
	"errors"

	"github.com/moby/buildkit/executor"
)

func newExecutor(_ string) (executor.Executor, error) {
	return nil, errors.New("buildkit executor not implemented for windows")
}

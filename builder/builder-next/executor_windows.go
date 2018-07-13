package buildkit

import (
	"context"
	"errors"
	"io"

	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/executor"
)

func newExecutor(_ string) (executor.Executor, error) {
	return &winExecutor{}, nil
}

type winExecutor struct {
}

func (e *winExecutor) Exec(ctx context.Context, meta executor.Meta, rootfs cache.Mountable, mounts []executor.Mount, stdin io.ReadCloser, stdout, stderr io.WriteCloser) error {
	return errors.New("buildkit executor not implemented for windows")
}

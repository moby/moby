package buildkit

import (
	"context"
	"errors"
	"io"

	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/libnetwork"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/executor"
)

func newExecutor(_, _ string, _ libnetwork.NetworkController, _ bool, _ *idtools.IdentityMapping) (executor.Executor, error) {
	return &winExecutor{}, nil
}

type winExecutor struct {
}

func (e *winExecutor) Exec(ctx context.Context, meta executor.Meta, rootfs cache.Mountable, mounts []executor.Mount, stdin io.ReadCloser, stdout, stderr io.WriteCloser) error {
	return errors.New("buildkit executor not implemented for windows")
}

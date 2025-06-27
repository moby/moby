//go:build !linux

package daemon

import (
	"context"

	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/daemon/container"
	"github.com/docker/docker/daemon/internal/libcontainerd/types"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// initializeCreatedTask performs any initialization that needs to be done to
// prepare a freshly-created task to be started.
func (daemon *Daemon) initializeCreatedTask(ctx context.Context, cfg *config.Config, tsk types.Task, container *container.Container, spec *specs.Spec) error {
	return nil
}

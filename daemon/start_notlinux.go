//go:build !linux

package daemon

import (
	"context"

	"github.com/moby/moby/v2/daemon/config"
	"github.com/moby/moby/v2/daemon/container"
	"github.com/moby/moby/v2/daemon/internal/libcontainerd/types"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// initializeCreatedTask performs any initialization that needs to be done to
// prepare a freshly-created task to be started.
func (daemon *Daemon) initializeCreatedTask(ctx context.Context, cfg *config.Config, tsk types.Task, container *container.Container, spec *specs.Spec) error {
	return nil
}

package daemon // import "github.com/docker/docker/daemon"

import (
	"context"

	"github.com/docker/swarmkit/agent/exec"
)

// SetContainerDependencyStore sets the dependency store backend for the container
func (daemon *Daemon) SetContainerDependencyStore(ctx context.Context, name string, store exec.DependencyGetter) error {
	c, err := daemon.GetContainer(ctx, name)
	if err != nil {
		return err
	}

	c.DependencyStore = store

	return nil
}

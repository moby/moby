package daemon

import (
	"github.com/docker/docker/context"
	derr "github.com/docker/docker/errors"
)

// ContainerUnpause unpauses a container
func (daemon *Daemon) ContainerUnpause(ctx context.Context, name string) error {
	container, err := daemon.Get(ctx, name)
	if err != nil {
		return err
	}

	if err := container.unpause(ctx); err != nil {
		return derr.ErrorCodeCantUnpause.WithArgs(name, err)
	}

	return nil
}

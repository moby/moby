package daemon

import (
	"github.com/docker/docker/context"
	derr "github.com/docker/docker/errors"
)

// ContainerPause pauses a container
func (daemon *Daemon) ContainerPause(ctx context.Context, name string) error {
	container, err := daemon.Get(ctx, name)
	if err != nil {
		return err
	}

	if err := container.pause(ctx); err != nil {
		return derr.ErrorCodePauseError.WithArgs(name, err)
	}

	return nil
}

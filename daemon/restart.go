package daemon

import (
	derr "github.com/docker/docker/errors"
)

// ContainerRestart stops and starts a container. It attempts to
// gracefully stop the container within the given timeout, forcefully
// stopping it if the timeout is exceeded. If given a negative
// timeout, ContainerRestart will wait forever until a graceful
// stop. Returns an error if the container cannot be found, or if
// there is an underlying error at any stage of the restart.
func (daemon *Daemon) ContainerRestart(name string, seconds int) error {
	container, err := daemon.Get(name)
	if err != nil {
		return err
	}
	if err := container.Restart(seconds); err != nil {
		return derr.ErrorCodeCantRestart.WithArgs(name, err)
	}
	return nil
}

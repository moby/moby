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
	if err := daemon.containerRestart(container, seconds); err != nil {
		return derr.ErrorCodeCantRestart.WithArgs(name, err)
	}
	return nil
}

// containerRestart attempts to gracefully stop and then start the
// container. When stopping, wait for the given duration in seconds to
// gracefully stop, before forcefully terminating the container. If
// given a negative duration, wait forever for a graceful stop.
func (daemon *Daemon) containerRestart(container *Container, seconds int) error {
	// Avoid unnecessarily unmounting and then directly mounting
	// the container when the container stops and then starts
	// again
	if err := daemon.Mount(container); err == nil {
		defer daemon.Unmount(container)
	}

	if err := daemon.containerStop(container, seconds); err != nil {
		return err
	}

	if err := daemon.containerStart(container); err != nil {
		return err
	}

	daemon.LogContainerEvent(container, "restart")
	return nil
}

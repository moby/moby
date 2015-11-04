package daemon

import (
	derr "github.com/docker/docker/errors"
)

// ContainerUnpause unpauses a container
func (daemon *Daemon) ContainerUnpause(name string) error {
	container, err := daemon.Get(name)
	if err != nil {
		return err
	}

	if err := daemon.containerUnpause(container); err != nil {
		return derr.ErrorCodeCantUnpause.WithArgs(name, err)
	}

	return nil
}

// containerUnpause resumes the container execution after the container is paused.
func (daemon *Daemon) containerUnpause(container *Container) error {
	container.Lock()
	defer container.Unlock()

	// We cannot unpause the container which is not running
	if !container.Running {
		return derr.ErrorCodeNotRunning.WithArgs(container.ID)
	}

	// We cannot unpause the container which is not paused
	if !container.Paused {
		return derr.ErrorCodeNotPaused.WithArgs(container.ID)
	}

	if err := daemon.execDriver.Unpause(container.command); err != nil {
		return err
	}

	container.Paused = false
	daemon.LogContainerEvent(container, "unpause")
	return nil
}

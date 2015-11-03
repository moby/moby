package daemon

import (
	derr "github.com/docker/docker/errors"
)

// ContainerPause pauses a container
func (daemon *Daemon) ContainerPause(name string) error {
	container, err := daemon.Get(name)
	if err != nil {
		return err
	}

	if err := daemon.containerPause(container); err != nil {
		return derr.ErrorCodePauseError.WithArgs(name, err)
	}

	return nil
}

// containerPause pauses the container execution without stopping the process.
// The execution can be resumed by calling containerUnpause.
func (daemon *Daemon) containerPause(container *Container) error {
	container.Lock()
	defer container.Unlock()

	// We cannot Pause the container which is not running
	if !container.Running {
		return derr.ErrorCodeNotRunning.WithArgs(container.ID)
	}

	// We cannot Pause the container which is already paused
	if container.Paused {
		return derr.ErrorCodeAlreadyPaused.WithArgs(container.ID)
	}

	if err := daemon.execDriver.Pause(container.command); err != nil {
		return err
	}
	container.Paused = true
	daemon.LogContainerEvent(container, "pause")
	return nil
}

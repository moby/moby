package daemon

import derr "github.com/docker/docker/errors"

// ContainerResize changes the size of the TTY of the process running
// in the container with the given name to the given height and width.
func (daemon *Daemon) ContainerResize(name string, height, width int) error {
	container, err := daemon.Get(name)
	if err != nil {
		return err
	}

	if !container.IsRunning() {
		return derr.ErrorCodeNotRunning.WithArgs(container.ID)
	}

	if err = container.Resize(height, width); err == nil {
		daemon.LogContainerEvent(container, "resize")
	}
	return err
}

// ContainerExecResize changes the size of the TTY of the process
// running in the exec with the given name to the given height and
// width.
func (daemon *Daemon) ContainerExecResize(name string, height, width int) error {
	ExecConfig, err := daemon.getExecConfig(name)
	if err != nil {
		return err
	}

	return ExecConfig.Resize(height, width)
}

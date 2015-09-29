package daemon

// ContainerResize changes the size of the TTY of the process running
// in the container with the given name to the given height and width.
func (daemon *Daemon) ContainerResize(name string, height, width int) error {
	container, err := daemon.Get(name)
	if err != nil {
		return err
	}

	return container.Resize(height, width)
}

// ContainerExecResize changes the size of the TTY of the process
// running in the exec with the given name to the given height and
// width.
func (daemon *Daemon) ContainerExecResize(name string, height, width int) error {
	ExecConfig, err := daemon.getExecConfig(name)
	if err != nil {
		return err
	}

	return ExecConfig.resize(height, width)
}

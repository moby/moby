package daemon

func (daemon *Daemon) ContainerExecResize(name string, height, width int) error {
	execConfig, err := daemon.getExecConfig(name)
	if err != nil {
		return err
	}
	if err := execConfig.Resize(height, width); err != nil {
		return err
	}
	return nil
}

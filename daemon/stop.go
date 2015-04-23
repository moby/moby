package daemon

import "fmt"

func (daemon *Daemon) ContainerStop(name string, seconds int) error {
	container, err := daemon.Get(name)
	if err != nil {
		return err
	}
	if err := container.Stop(seconds); err != nil {
		return fmt.Errorf("Cannot stop container %s: %s\n", name, err)
	}
	container.LogEvent("stop")
	return nil
}

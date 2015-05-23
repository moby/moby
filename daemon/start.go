package daemon

import (
	"fmt"

	"github.com/docker/docker/runconfig"
)

func (daemon *Daemon) ContainerStart(name string, hostConfig *runconfig.HostConfig) error {
	container, err := daemon.Get(name)
	if err != nil {
		return err
	}

	// This is kept for backward compatibility - hostconfig should be passed when
	// creating a container, not during start.
	if _, err := container.SetHostConfig(hostConfig); err != nil {
		return err
	}

	if err := container.Start(); err != nil {
		container.LogEvent("die")
		return fmt.Errorf("Cannot start container %s: %s", name, err)
	}

	return nil
}

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

	if container.IsPaused() {
		return fmt.Errorf("Cannot start a paused container, try unpause instead.")
	}

	if container.IsRunning() {
		return fmt.Errorf("Container already started")
	}

	if _, err = daemon.verifyContainerSettings(hostConfig, nil); err != nil {
		return err
	}

	// This is kept for backward compatibility - hostconfig should be passed when
	// creating a container, not during start.
	if hostConfig != nil {
		if err := daemon.setHostConfig(container, hostConfig); err != nil {
			return err
		}
	}

	if err := container.Start(); err != nil {
		return fmt.Errorf("Cannot start container %s: %s", name, err)
	}

	return nil
}

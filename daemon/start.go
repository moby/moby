package daemon

import (
	"runtime"

	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
)

// ContainerStart starts a container.
func (daemon *Daemon) ContainerStart(name string, hostConfig *runconfig.HostConfig) error {
	container, err := daemon.Get(name)
	if err != nil {
		return err
	}

	if container.isPaused() {
		return derr.ErrorCodeStartPaused
	}

	if container.IsRunning() {
		return derr.ErrorCodeAlreadyStarted
	}

	// Windows does not have the backwards compatibility issue here.
	if runtime.GOOS != "windows" {
		// This is kept for backward compatibility - hostconfig should be passed when
		// creating a container, not during start.
		if hostConfig != nil {
			if err := daemon.setHostConfig(container, hostConfig); err != nil {
				return err
			}
		}
	} else {
		if hostConfig != nil {
			return derr.ErrorCodeHostConfigStart
		}
	}

	// check if hostConfig is in line with the current system settings.
	// It may happen cgroups are umounted or the like.
	if _, err = daemon.verifyContainerSettings(container.hostConfig, nil); err != nil {
		return err
	}

	if err := container.Start(); err != nil {
		return derr.ErrorCodeCantStart.WithArgs(name, utils.GetErrorMessage(err))
	}

	return nil
}

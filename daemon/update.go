package daemon

import (
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

// ContainerUpdate updates configuration of the container
func (daemon *Daemon) ContainerUpdate(name string, hostConfig *container.HostConfig, validateHostname bool) (types.ContainerUpdateResponse, error) {
	var warnings []string

	warnings, err := daemon.verifyContainerSettings(hostConfig, nil, true, validateHostname)
	if err != nil {
		return types.ContainerUpdateResponse{Warnings: warnings}, err
	}

	if err := daemon.update(name, hostConfig); err != nil {
		return types.ContainerUpdateResponse{Warnings: warnings}, err
	}

	return types.ContainerUpdateResponse{Warnings: warnings}, nil
}

// ContainerUpdateCmdOnBuild updates Path and Args for the container with ID cID.
func (daemon *Daemon) ContainerUpdateCmdOnBuild(cID string, cmd []string) error {
	if len(cmd) == 0 {
		return nil
	}
	c, err := daemon.GetContainer(cID)
	if err != nil {
		return err
	}
	c.Path = cmd[0]
	c.Args = cmd[1:]
	return nil
}

func (daemon *Daemon) update(name string, hostConfig *container.HostConfig) error {
	if hostConfig == nil {
		return nil
	}

	container, err := daemon.GetContainer(name)
	if err != nil {
		return err
	}

	restoreConfig := false
	backupHostConfig := *container.HostConfig
	defer func() {
		if restoreConfig {
			container.Lock()
			container.HostConfig = &backupHostConfig
			container.ToDisk()
			container.Unlock()
		}
	}()

	if container.RemovalInProgress || container.Dead {
		return errCannotUpdate(container.ID, fmt.Errorf("Container is marked for removal and cannot be \"update\"."))
	}

	if err := container.UpdateContainer(hostConfig); err != nil {
		restoreConfig = true
		return errCannotUpdate(container.ID, err)
	}

	// if Restart Policy changed, we need to update container monitor
	container.UpdateMonitor(hostConfig.RestartPolicy)

	// If container is not running, update hostConfig struct is enough,
	// resources will be updated when the container is started again.
	// If container is running (including paused), we need to update configs
	// to the real world.
	if container.IsRunning() && !container.IsRestarting() {
		if err := daemon.containerd.UpdateResources(container.ID, toContainerdResources(hostConfig.Resources)); err != nil {
			restoreConfig = true
			return errCannotUpdate(container.ID, err)
		}
	}

	daemon.LogContainerEvent(container, "update")

	return nil
}

func errCannotUpdate(containerID string, err error) error {
	return fmt.Errorf("Cannot update container %s: %v", containerID, err)
}

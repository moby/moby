package daemon

import (
	"fmt"

	"github.com/docker/engine-api/types/container"
)

// ContainerUpdate updates resources of the container
func (daemon *Daemon) ContainerUpdate(name string, hostConfig *container.HostConfig) ([]string, error) {
	var warnings []string

	warnings, err := daemon.verifyContainerSettings(hostConfig, nil)
	if err != nil {
		return warnings, err
	}

	if err := daemon.update(name, hostConfig); err != nil {
		return warnings, err
	}

	return warnings, nil
}

// ContainerUpdateCmdOnBuild updates Path and Args for the container with ID cID.
func (daemon *Daemon) ContainerUpdateCmdOnBuild(cID string, cmd []string) error {
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

	if container.RemovalInProgress || container.Dead {
		return fmt.Errorf("Container is marked for removal and cannot be \"update\".")
	}

	if container.IsRunning() && hostConfig.KernelMemory != 0 {
		return fmt.Errorf("Can not update kernel memory to a running container, please stop it first.")
	}

	if err := container.UpdateContainer(hostConfig); err != nil {
		return err
	}

	// If container is not running, update hostConfig struct is enough,
	// resources will be updated when the container is started again.
	// If container is running (including paused), we need to update configs
	// to the real world.
	if container.IsRunning() {
		if err := daemon.execDriver.Update(container.Command); err != nil {
			return err
		}
	}

	daemon.LogContainerEvent(container, "update")

	return nil
}

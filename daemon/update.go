package daemon

import (
	"fmt"
	"time"

	derr "github.com/docker/docker/errors"
	"github.com/docker/engine-api/types/container"
)

// ContainerUpdate updates configuration of the container
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
		errMsg := fmt.Errorf("Container is marked for removal and cannot be \"update\".")
		return derr.ErrorCodeCantUpdate.WithArgs(container.ID, errMsg)
	}

	if container.IsRunning() && hostConfig.KernelMemory != 0 {
		errMsg := fmt.Errorf("Can not update kernel memory to a running container, please stop it first.")
		return derr.ErrorCodeCantUpdate.WithArgs(container.ID, errMsg)
	}

	if err := container.UpdateContainer(hostConfig); err != nil {
		return derr.ErrorCodeCantUpdate.WithArgs(container.ID, err.Error())
	}

	// if Restart Policy changed, we need to update container monitor
	container.UpdateMonitor(hostConfig.RestartPolicy)

	// if container is restarting, wait 5 seconds until it's running
	if container.IsRestarting() {
		container.WaitRunning(5 * time.Second)
	}

	// If container is not running, update hostConfig struct is enough,
	// resources will be updated when the container is started again.
	// If container is running (including paused), we need to update configs
	// to the real world.
	if container.IsRunning() && !container.IsRestarting() {
		if err := daemon.execDriver.Update(container.Command); err != nil {
			return derr.ErrorCodeCantUpdate.WithArgs(container.ID, err.Error())
		}
	}

	daemon.LogContainerEvent(container, "update")

	return nil
}

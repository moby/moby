// +build !windows

package daemon

import (
	"fmt"

	"github.com/docker/docker/container"
	"github.com/docker/docker/libcontainerd"
)

func (daemon *Daemon) getLibcontainerdCreateOptions(container *container.Container) ([]libcontainerd.CreateOption, error) {
	createOptions := []libcontainerd.CreateOption{}

	// Ensure a runtime has been assigned to this container
	if container.HostConfig.Runtime == "" {
		container.HostConfig.Runtime = daemon.runtimeManager.GetDefaultRuntimeName()
		container.ToDisk()
	}

	path, args, err := daemon.runtimeManager.GetRuntimePathAndArgs(container.HostConfig.Runtime)
	if err != nil {
		return nil, fmt.Errorf("no such runtime '%s'", container.HostConfig.Runtime)
	}
	if UsingSystemd(daemon.configStore) {
		args = append(args, "--systemd-cgroup=true")
	}

	createOptions = append(createOptions, libcontainerd.WithRuntime(path, args))

	return createOptions, nil
}

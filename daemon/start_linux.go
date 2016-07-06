package daemon

import (
	"fmt"

	"github.com/docker/docker/container"
	"github.com/docker/docker/libcontainerd"
)

func (daemon *Daemon) getLibcontainerdCreateOptions(container *container.Container) (*[]libcontainerd.CreateOption, error) {
	createOptions := []libcontainerd.CreateOption{}
	runtimeName := container.HostConfig.Runtime

	// Ensure a runtime has been assigned to this container
	if runtimeName == "" {
		runtimeName = stockRuntimeName
	}

	rt := daemon.configStore.GetRuntime(runtimeName)
	if rt == nil {
		return nil, fmt.Errorf("no such runtime '%s'", runtimeName)
	}
	createOptions = append(createOptions, libcontainerd.WithRuntime(rt.Path, rt.Args))

	return &createOptions, nil
}

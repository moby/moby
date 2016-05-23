package daemon

import (
	"fmt"

	"github.com/docker/docker/container"
	"github.com/docker/docker/libcontainerd"
)

func (daemon *Daemon) getLibcontainerdCreateOptions(container *container.Container) (*[]libcontainerd.CreateOption, error) {
	createOptions := []libcontainerd.CreateOption{}

	rt := daemon.configStore.GetRuntime(container.HostConfig.Runtime)
	if rt == nil {
		return nil, fmt.Errorf("No such runtime '%s'", container.HostConfig.Runtime)
	}
	createOptions = append(createOptions, libcontainerd.WithRuntime(rt.Path, rt.Args))

	return &createOptions, nil
}

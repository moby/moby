package daemon

import (
	"github.com/docker/docker/container"
	"github.com/docker/docker/libcontainerd"
)

func (daemon *Daemon) getLibcontainerdCreateOptions(container *container.Container) (*[]libcontainerd.CreateOption, error) {
	return &[]libcontainerd.CreateOption{}, nil
}

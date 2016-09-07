// +build !experimental

package daemon

import (
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/libcontainerd"
)

func (daemon *Daemon) verifyExperimentalContainerSettings(hostConfig *container.HostConfig, config *container.Config) ([]string, error) {
	return nil, nil
}

func pluginInit(d *Daemon, config *Config, remote libcontainerd.Remote) error {
	return nil
}

func pluginShutdown() {
}

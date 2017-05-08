// +build !experimental

package daemon

import (
	"github.com/docker/docker/libcontainerd"
	"github.com/docker/engine-api/types/container"
)

func (daemon *Daemon) verifyExperimentalContainerSettings(hostConfig *container.HostConfig, config *container.Config) ([]string, error) {
	return nil, nil
}

func pluginInit(d *Daemon, config *Config, remote libcontainerd.Remote) error {
	return nil
}

func pluginShutdown() {
}

package daemon

import (
	"github.com/containerd/containerd/v2/defaults"
	"github.com/docker/docker/daemon/container"
)

func (daemon *Daemon) getLibcontainerdCreateOptions(daemonCfg *configStore, container *container.Container) (string, interface{}, error) {
	if container.HostConfig.Runtime != "" {
		return container.HostConfig.Runtime, nil, nil
	}

	if daemonCfg.DefaultRuntime != "" {
		return daemonCfg.DefaultRuntime, nil, nil
	}

	if daemon.containerdClient == nil {
		// We're running in legacy non-containerd mode, runtime doesn't affect anything
		return "", nil, nil
	}

	return defaults.DefaultRuntime, nil, nil
}

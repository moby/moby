package daemon

import (
	"context"

	"github.com/containerd/containerd/v2/defaults"
	"github.com/moby/moby/v2/daemon/container"
)

func (daemon *Daemon) getLibcontainerdCreateOptions(daemonCfg *configStore, container *container.Container) (string, any, error) {
	if container.HostConfig.Runtime == "" {
		if daemonCfg.DefaultRuntime != "" {
			container.HostConfig.Runtime = daemonCfg.DefaultRuntime
		} else {
			container.HostConfig.Runtime = defaults.DefaultRuntime
		}

		container.CheckpointTo(context.WithoutCancel(context.TODO()), daemon.containersReplica)
	}

	return container.HostConfig.Runtime, nil, nil
}

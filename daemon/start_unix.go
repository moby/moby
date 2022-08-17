//go:build !windows

package daemon // import "github.com/docker/docker/daemon"

import (
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/config"
)

// getLibcontainerdCreateOptions callers must hold a lock on the container
func (daemon *Daemon) getLibcontainerdCreateOptions(daemonCfg *config.Config, container *container.Container) (string, interface{}, error) {
	// Ensure a runtime has been assigned to this container
	if container.HostConfig.Runtime == "" {
		container.HostConfig.Runtime = daemonCfg.DefaultRuntime
		container.CheckpointTo(daemon.containersReplica)
	}

	binary, opts, err := daemon.getRuntime(daemonCfg, container.HostConfig.Runtime)
	if err != nil {
		return "", nil, setExitCodeFromError(container.SetExitCode, err)
	}

	return binary, opts, nil
}

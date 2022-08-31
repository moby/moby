//go:build !windows

package daemon // import "github.com/docker/docker/daemon"

import (
	"github.com/docker/docker/container"
)

// getLibcontainerdCreateOptions callers must hold a lock on the container
func (daemon *Daemon) getLibcontainerdCreateOptions(daemonCfg *configStore, container *container.Container) (string, interface{}, error) {
	// Ensure a runtime has been assigned to this container
	if container.HostConfig.Runtime == "" {
		container.HostConfig.Runtime = daemonCfg.DefaultRuntime
		container.CheckpointTo(daemon.containersReplica)
	}

	shim, opts, err := daemonCfg.Runtimes.Get(container.HostConfig.Runtime)
	if err != nil {
		return "", nil, setExitCodeFromError(container.SetExitCode, err)
	}

	return shim, opts, nil
}

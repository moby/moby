//go:build !windows
// +build !windows

package daemon // import "github.com/docker/docker/daemon"

import (
	"github.com/docker/docker/container"
)

// getLibcontainerdCreateOptions callers must hold a lock on the container
func (daemon *Daemon) getLibcontainerdCreateOptions(container *container.Container) (string, interface{}, error) {
	// Ensure a runtime has been assigned to this container
	if container.HostConfig.Runtime == "" {
		container.HostConfig.Runtime = daemon.configStore.GetDefaultRuntimeName()
		container.CheckpointTo(daemon.containersReplica)
	}

	rt, err := daemon.getRuntime(container.HostConfig.Runtime)
	if err != nil {
		return "", nil, translateContainerdStartErr(container.Path, container.SetExitCode, err)
	}

	return rt.Shim.Binary, rt.Shim.Opts, nil
}

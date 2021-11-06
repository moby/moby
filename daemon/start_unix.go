//go:build !windows
// +build !windows

package daemon // import "github.com/docker/docker/daemon"

import (
	typeurl "github.com/containerd/typeurl"
	"github.com/docker/docker/container"
	"github.com/sirupsen/logrus"
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

	opts, err := typeurl.UnmarshalAny(rt.Shim.Opts)
	if err != nil {
		logrus.Errorf("Fail to UnmarshalAny option for runtime shim: %v", err)
	}
	return rt.Shim.Binary, opts, nil
}

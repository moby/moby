// +build !windows

package daemon // import "github.com/docker/docker/daemon"

import (
	"github.com/docker/docker/container"
	"github.com/docker/docker/errdefs"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// getLibcontainerdCreateOptions callers must hold a lock on the container
func (daemon *Daemon) getLibcontainerdCreateOptions(container *container.Container) (string, interface{}, error) {
	// Ensure a runtime has been assigned to this container
	if container.HostConfig.Runtime == "" {
		container.HostConfig.Runtime = daemon.configStore.GetDefaultRuntimeName()
		container.CheckpointTo(daemon.containersReplica)
	}

	rt := daemon.configStore.GetRuntime(container.HostConfig.Runtime)
	if rt.Shim == nil {
		p, err := daemon.rewriteRuntimePath(container.HostConfig.Runtime, rt.Path, rt.Args)
		if err != nil {
			return "", nil, translateContainerdStartErr(container.Path, container.SetExitCode, err)
		}
		rt.Shim = defaultV2ShimConfig(daemon.configStore, p)
	}
	if rt.Shim.Binary == linuxShimV1 {
		if cgroups.IsCgroup2UnifiedMode() {
			return "", nil, errdefs.InvalidParameter(errors.Errorf("runtime %q is not supported while cgroups v2 (unified hierarchy) is being used", container.HostConfig.Runtime))
		}
		logrus.Warnf("Configured runtime %q is deprecated and will be removed in the next release", container.HostConfig.Runtime)
	}

	return rt.Shim.Binary, rt.Shim.Opts, nil
}

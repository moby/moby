package daemon

import (
	"github.com/Microsoft/hcsshim/cmd/containerd-shim-runhcs-v1/options"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/daemon/container"
	"github.com/docker/docker/daemon/internal/libcontainerd"
)

func (daemon *Daemon) getLibcontainerdCreateOptions(*configStore, *container.Container) (string, interface{}, error) {
	if libcontainerd.ContainerdRuntimeEnabled {
		opts := &options.Options{}
		return config.WindowsV2RuntimeName, opts, nil
	}
	return "", nil, nil
}

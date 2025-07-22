package daemon

import (
	"github.com/Microsoft/hcsshim/cmd/containerd-shim-runhcs-v1/options"
	"github.com/moby/moby/daemon/config"
	"github.com/moby/moby/daemon/container"
	"github.com/moby/moby/daemon/internal/libcontainerd"
)

func (daemon *Daemon) getLibcontainerdCreateOptions(*configStore, *container.Container) (string, interface{}, error) {
	if libcontainerd.ContainerdRuntimeEnabled {
		opts := &options.Options{}
		return config.WindowsV2RuntimeName, opts, nil
	}
	return "", nil, nil
}

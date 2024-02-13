package daemon // import "github.com/docker/docker/daemon"

import (
	"github.com/Microsoft/hcsshim/cmd/containerd-shim-runhcs-v1/options"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/pkg/system"
)

func (daemon *Daemon) getLibcontainerdCreateOptions(*configStore, *container.Container) (string, interface{}, error) {
	if system.ContainerdRuntimeSupported() {
		opts := &options.Options{}
		return config.WindowsV2RuntimeName, opts, nil
	}
	return "", nil, nil
}

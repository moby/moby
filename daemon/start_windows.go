package daemon // import "github.com/moby/moby/daemon"

import (
	"github.com/Microsoft/hcsshim/cmd/containerd-shim-runhcs-v1/options"
	"github.com/moby/moby/container"
	"github.com/moby/moby/pkg/system"
)

func (daemon *Daemon) getLibcontainerdCreateOptions(_ *container.Container) (string, interface{}, error) {
	if system.ContainerdRuntimeSupported() {
		opts := &options.Options{}
		return "io.containerd.runhcs.v1", opts, nil
	}
	return "", nil, nil
}

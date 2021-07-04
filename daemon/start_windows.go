package daemon // import "github.com/docker/docker/daemon"

import (
	"github.com/Microsoft/hcsshim/cmd/containerd-shim-runhcs-v1/options"
	"github.com/docker/docker/container"
	"github.com/docker/docker/pkg/system"
)

func (daemon *Daemon) getLibcontainerdCreateOptions(_ *container.Container) (string, interface{}, error) {
	if system.ContainerdRuntimeSupported() {
		// Set the runtime options to debug regardless of current logging level.
		return "", &options.Options{Debug: true}, nil
	}
	return "", nil, nil
}

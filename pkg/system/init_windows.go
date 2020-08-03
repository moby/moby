package system // import "github.com/docker/docker/pkg/system"

import (
	"os"

	"github.com/sirupsen/logrus"
)

var (
	// containerdRuntimeSupported determines if ContainerD should be the runtime.
	// As of March 2019, this is an experimental feature.
	containerdRuntimeSupported = false
)

// InitContainerdRuntime sets whether to use ContainerD for runtime
// on Windows. This is an experimental feature still in development, and
// also requires an environment variable to be set (so as not to turn the
// feature on from simply experimental which would also mean LCOW.
func InitContainerdRuntime(experimental bool, cdPath string) {
	if experimental && len(cdPath) > 0 && len(os.Getenv("DOCKER_WINDOWS_CONTAINERD_RUNTIME")) > 0 {
		logrus.Warnf("Using ContainerD runtime. This feature is experimental")
		containerdRuntimeSupported = true
	}
}

// ContainerdRuntimeSupported returns true if the use of ContainerD runtime is supported.
func ContainerdRuntimeSupported() bool {
	return containerdRuntimeSupported
}

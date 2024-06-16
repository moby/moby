package runconfig // import "github.com/docker/docker/runconfig"

import (
	"github.com/docker/docker/api/types/container"
)

// getHostConfig gets the HostConfig of the Config.
func (w *ContainerConfigWrapper) getHostConfig() *container.HostConfig {
	return w.HostConfig
}

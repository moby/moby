//go:build !windows

package runconfig // import "github.com/docker/docker/runconfig"

import "github.com/docker/docker/api/types/container"

// getHostConfig gets the HostConfig of the Config.
// It's mostly there to handle Deprecated fields of the ContainerConfigWrapper
func (w *ContainerConfigWrapper) getHostConfig() *container.HostConfig {
	hc := w.HostConfig

	// Make sure NetworkMode has an acceptable value. We do this to ensure
	// backwards compatible API behavior.
	SetDefaultNetModeIfBlank(hc)

	return hc
}

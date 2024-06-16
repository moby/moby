//go:build !windows

package runconfig // import "github.com/docker/docker/runconfig"

import (
	"github.com/docker/docker/api/types/container"
	networktypes "github.com/docker/docker/api/types/network"
)

// ContainerConfigWrapper is a Config wrapper that holds the container Config (portable)
// and the corresponding HostConfig (non-portable).
type ContainerConfigWrapper struct {
	*container.Config
	HostConfig       *container.HostConfig          `json:"HostConfig,omitempty"`
	NetworkingConfig *networktypes.NetworkingConfig `json:"NetworkingConfig,omitempty"`
}

// getHostConfig gets the HostConfig of the Config.
// It's mostly there to handle Deprecated fields of the ContainerConfigWrapper
func (w *ContainerConfigWrapper) getHostConfig() *container.HostConfig {
	hc := w.HostConfig

	// Make sure NetworkMode has an acceptable value. We do this to ensure
	// backwards compatible API behavior.
	SetDefaultNetModeIfBlank(hc)

	return hc
}

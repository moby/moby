package runconfig // import "github.com/docker/docker/runconfig"

import (
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
)

// SetDefaultNetModeIfBlank changes the NetworkMode in a HostConfig structure
// to default if it is not populated. This ensures backwards compatibility after
// the validation of the network mode was moved from the docker CLI to the
// docker daemon.
//
// Deprecated: SetDefaultNetModeIfBlank is no longer used abnd will be removed in the next release.
func SetDefaultNetModeIfBlank(hc *container.HostConfig) {
	if hc != nil && hc.NetworkMode == "" {
		hc.NetworkMode = network.NetworkDefault
	}
}

// validateNetContainerMode ensures that the various combinations of requested
// network settings wrt container mode are valid.
func validateNetContainerMode(c *container.Config, hc *container.HostConfig) error {
	parts := strings.Split(string(hc.NetworkMode), ":")
	if parts[0] == "container" {
		if len(parts) < 2 || parts[1] == "" {
			return validationError("Invalid network mode: invalid container format container:<name|id>")
		}
	}

	if hc.NetworkMode.IsContainer() && c.Hostname != "" {
		return ErrConflictNetworkHostname
	}

	if hc.NetworkMode.IsContainer() && len(hc.Links) > 0 {
		return ErrConflictContainerNetworkAndLinks
	}

	if hc.NetworkMode.IsContainer() && len(hc.DNS) > 0 {
		return ErrConflictNetworkAndDNS
	}

	if hc.NetworkMode.IsContainer() && len(hc.ExtraHosts) > 0 {
		return ErrConflictNetworkHosts
	}

	if hc.NetworkMode.IsContainer() && (len(hc.PortBindings) > 0 || hc.PublishAllPorts) {
		return ErrConflictNetworkPublishPorts
	}

	if hc.NetworkMode.IsContainer() && len(c.ExposedPorts) > 0 {
		return ErrConflictNetworkExposePorts
	}
	return nil
}

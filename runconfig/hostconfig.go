package runconfig // import "github.com/docker/docker/runconfig"

import (
	"github.com/docker/docker/api/types/container"
)

// validateNetContainerMode ensures that the various combinations of requested
// network settings wrt container mode are valid.
func validateNetContainerMode(c *container.Config, hc *container.HostConfig) error {
	// FIXME(thaJeztah): a network named "container" (without colon) is not seen as "container-mode" network.
	if string(hc.NetworkMode) != "container" && !hc.NetworkMode.IsContainer() {
		return nil
	}

	if hc.NetworkMode.ConnectedContainer() == "" {
		return validationError("Invalid network mode: invalid container format container:<name|id>")
	}

	if c.Hostname != "" {
		return ErrConflictNetworkHostname
	}

	if len(hc.Links) > 0 {
		return ErrConflictContainerNetworkAndLinks
	}

	if len(hc.DNS) > 0 {
		return ErrConflictNetworkAndDNS
	}

	if len(hc.ExtraHosts) > 0 {
		return ErrConflictNetworkHosts
	}

	if len(hc.PortBindings) > 0 || hc.PublishAllPorts {
		return ErrConflictNetworkPublishPorts
	}

	if len(c.ExposedPorts) > 0 {
		return ErrConflictNetworkExposePorts
	}
	return nil
}

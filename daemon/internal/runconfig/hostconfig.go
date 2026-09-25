package runconfig

import "github.com/moby/moby/api/types/container"

// validateNetContainerMode ensures that the various combinations of requested
// network settings wrt container mode are valid.
func validateNetContainerMode(c *container.Config, hc *container.HostConfig) error {
	// FIXME(thaJeztah): a network named "container" (without colon) is not seen as "container-mode" network.
	if string(hc.NetworkMode) != "container" && !hc.NetworkMode.IsContainer() {
		return nil
	}

	if hc.NetworkMode.ConnectedContainer() == "" {
		return validationError("invalid network mode: invalid container format container:<name|id>")
	}

	if c.Hostname != "" {
		return validationError("conflicting options: hostname and the network mode")
	}

	if len(hc.Links) > 0 {
		return validationError("conflicting options: container type network can't be used with links. This would result in undefined behavior")
	}

	if len(hc.DNS) > 0 {
		return validationError("conflicting options: dns and the network mode")
	}

	if len(hc.ExtraHosts) > 0 {
		return validationError("conflicting options: custom host-to-IP mapping and the network mode")
	}

	if len(hc.PortBindings) > 0 || hc.PublishAllPorts {
		return validationError("conflicting options: port publishing and the container type network mode")
	}

	if len(c.ExposedPorts) > 0 {
		return validationError("conflicting options: port exposing and the container type network mode")
	}
	return nil
}

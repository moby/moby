// +build !windows

package runconfig

import (
	"strings"

	derr "github.com/docker/docker/errors"
)

// ValidateNetMode ensures that the various combinations of requested
// network settings are valid.
func ValidateNetMode(c *Config, hc *HostConfig) error {
	// We may not be passed a host config, such as in the case of docker commit
	if hc == nil {
		return nil
	}
	parts := strings.Split(string(hc.NetworkMode), ":")
	switch mode := parts[0]; mode {
	case "default", "bridge", "none", "host":
	case "container":
		if len(parts) < 2 || parts[1] == "" {
			return derr.ErrorCodeInvalidNetworkFormat
		}
	default:
		return derr.ErrorCodeInvalidNetworkOption.WithArgs(hc.NetworkMode)
	}

	if (hc.NetworkMode.IsHost() || hc.NetworkMode.IsContainer()) && c.Hostname != "" {
		return derr.ErrorCodeConflictNetworkHostname
	}

	if hc.NetworkMode.IsHost() && len(hc.Links) > 0 {
		return derr.ErrorCodeConflictHostNetworkAndLinks
	}

	if hc.NetworkMode.IsContainer() && len(hc.Links) > 0 {
		return derr.ErrorCodeConflictContainerNetworkAndLinks
	}

	if (hc.NetworkMode.IsHost() || hc.NetworkMode.IsContainer()) && len(hc.DNS) > 0 {
		return derr.ErrorCodeConflictNetworkAndDNS
	}

	if (hc.NetworkMode.IsContainer() || hc.NetworkMode.IsHost()) && len(hc.ExtraHosts) > 0 {
		return derr.ErrorCodeConflictNetworkHosts
	}

	if (hc.NetworkMode.IsContainer() || hc.NetworkMode.IsHost()) && c.MacAddress != "" {
		return derr.ErrorCodeConflictContainerNetworkAndMac
	}

	if hc.NetworkMode.IsContainer() && (len(hc.PortBindings) > 0 || hc.PublishAllPorts == true) {
		return derr.ErrorCodeConflictNetworkPublishPorts
	}

	if hc.NetworkMode.IsContainer() && len(c.ExposedPorts) > 0 {
		return derr.ErrorCodeConflictNetworkExposePorts
	}
	return nil
}

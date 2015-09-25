// +build !windows

package runconfig

import (
	"fmt"
	"strings"
)

// ValidateNetMode ensures that the various combinations of requested
// network settings are valid.
func ValidateNetMode(c *Config, hc *HostConfig) error {
	// We may not be passed a host config, such as in the case of docker commit
	if hc == nil {
		return nil
	}
	parts := strings.Split(string(hc.NetworkMode), ":")
	if parts[0] == "container" {
		if len(parts) < 2 || parts[1] == "" {
			return fmt.Errorf("--net: invalid net mode: invalid container format container:<name|id>")
		}
	}

	if (hc.NetworkMode.IsHost() || hc.NetworkMode.IsContainer()) && c.Hostname != "" {
		return ErrConflictNetworkHostname
	}

	if hc.NetworkMode.IsHost() && len(hc.Links) > 0 {
		return ErrConflictHostNetworkAndLinks
	}

	if hc.NetworkMode.IsContainer() && len(hc.Links) > 0 {
		return ErrConflictContainerNetworkAndLinks
	}

	if hc.NetworkMode.IsUserDefined() && len(hc.Links) > 0 {
		return ErrConflictUserDefinedNetworkAndLinks
	}

	if (hc.NetworkMode.IsHost() || hc.NetworkMode.IsContainer()) && len(hc.DNS) > 0 {
		return ErrConflictNetworkAndDNS
	}

	if (hc.NetworkMode.IsContainer() || hc.NetworkMode.IsHost()) && len(hc.ExtraHosts) > 0 {
		return ErrConflictNetworkHosts
	}

	if (hc.NetworkMode.IsContainer() || hc.NetworkMode.IsHost()) && c.MacAddress != "" {
		return ErrConflictContainerNetworkAndMac
	}

	if hc.NetworkMode.IsContainer() && (len(hc.PortBindings) > 0 || hc.PublishAllPorts == true) {
		return ErrConflictNetworkPublishPorts
	}

	if hc.NetworkMode.IsContainer() && len(c.ExposedPorts) > 0 {
		return ErrConflictNetworkExposePorts
	}
	return nil
}

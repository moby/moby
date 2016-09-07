package runconfig

import (
	"fmt"
	"strings"

	"github.com/docker/docker/api/types/container"
)

// DefaultDaemonNetworkMode returns the default network stack the daemon should
// use.
func DefaultDaemonNetworkMode() container.NetworkMode {
	return container.NetworkMode("default")
}

// IsPreDefinedNetwork indicates if a network is predefined by the daemon
func IsPreDefinedNetwork(network string) bool {
	return false
}

// ValidateNetMode ensures that the various combinations of requested
// network settings are valid.
func ValidateNetMode(c *container.Config, hc *container.HostConfig) error {
	// We may not be passed a host config, such as in the case of docker commit
	if hc == nil {
		return nil
	}
	parts := strings.Split(string(hc.NetworkMode), ":")
	switch mode := parts[0]; mode {
	case "default", "none":
	default:
		return fmt.Errorf("invalid --net: %s", hc.NetworkMode)
	}
	return nil
}

// ValidateIsolation performs platform specific validation of the
// isolation level in the hostconfig structure.
// This setting is currently discarded for Solaris so this is a no-op.
func ValidateIsolation(hc *container.HostConfig) error {
	return nil
}

// ValidateQoS performs platform specific validation of the QoS settings
func ValidateQoS(hc *container.HostConfig) error {
	return nil
}

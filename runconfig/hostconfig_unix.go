// +build !windows

package runconfig

import (
	"fmt"
	"runtime"
	"strings"
)

// IsValid indicates is an isolation level is valid
func (i IsolationLevel) IsValid() bool {
	return i.IsDefault()
}

// IsPrivate indicates whether container uses it's private network stack.
func (n NetworkMode) IsPrivate() bool {
	return !(n.IsHost() || n.IsContainer())
}

// IsDefault indicates whether container uses the default network stack.
func (n NetworkMode) IsDefault() bool {
	return n == "default"
}

// DefaultDaemonNetworkMode returns the default network stack the daemon should
// use.
func DefaultDaemonNetworkMode() NetworkMode {
	return NetworkMode("bridge")
}

// NetworkName returns the name of the network stack.
func (n NetworkMode) NetworkName() string {
	if n.IsBridge() {
		return "bridge"
	} else if n.IsHost() {
		return "host"
	} else if n.IsContainer() {
		return "container"
	} else if n.IsNone() {
		return "none"
	} else if n.IsDefault() {
		return "default"
	} else if n.IsUserDefined() {
		return n.UserDefined()
	}
	return ""
}

// IsBridge indicates whether container uses the bridge network stack
func (n NetworkMode) IsBridge() bool {
	return n == "bridge"
}

// IsHost indicates whether container uses the host network stack.
func (n NetworkMode) IsHost() bool {
	return n == "host"
}

// IsContainer indicates whether container uses a container network stack.
func (n NetworkMode) IsContainer() bool {
	parts := strings.SplitN(string(n), ":", 2)
	return len(parts) > 1 && parts[0] == "container"
}

// IsNone indicates whether container isn't using a network stack.
func (n NetworkMode) IsNone() bool {
	return n == "none"
}

// ConnectedContainer is the id of the container which network this container is connected to.
func (n NetworkMode) ConnectedContainer() string {
	parts := strings.SplitN(string(n), ":", 2)
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// IsUserDefined indicates user-created network
func (n NetworkMode) IsUserDefined() bool {
	return !n.IsDefault() && !n.IsBridge() && !n.IsHost() && !n.IsNone() && !n.IsContainer()
}

// IsPreDefinedNetwork indicates if a network is predefined by the daemon
func IsPreDefinedNetwork(network string) bool {
	n := NetworkMode(network)
	return n.IsBridge() || n.IsHost() || n.IsNone()
}

//UserDefined indicates user-created network
func (n NetworkMode) UserDefined() string {
	if n.IsUserDefined() {
		return string(n)
	}
	return ""
}

// MergeConfigs merges the specified container Config and HostConfig.
// It creates a ContainerConfigWrapper.
func MergeConfigs(config *Config, hostConfig *HostConfig) *ContainerConfigWrapper {
	return &ContainerConfigWrapper{
		config,
		hostConfig,
		"", nil,
	}
}

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

// ValidateIsolationLevel performs platform specific validation of the
// isolation level in the hostconfig structure. Linux only supports "default"
// which is LXC container isolation
func ValidateIsolationLevel(hc *HostConfig) error {
	// We may not be passed a host config, such as in the case of docker commit
	if hc == nil {
		return nil
	}
	if !hc.Isolation.IsValid() {
		return fmt.Errorf("invalid --isolation: %q - %s only supports 'default'", hc.Isolation, runtime.GOOS)
	}
	return nil
}

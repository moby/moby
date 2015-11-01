package runconfig

import (
	"fmt"
	"strings"
)

// IsDefault indicates whether container uses the default network stack.
func (n NetworkMode) IsDefault() bool {
	return n == "default"
}

// IsHyperV indicates the use of Hyper-V Containers for isolation (as opposed
// to Windows Server Containers
func (i IsolationLevel) IsHyperV() bool {
	return strings.ToLower(string(i)) == "hyperv"
}

// IsValid indicates is an isolation level is valid
func (i IsolationLevel) IsValid() bool {
	return i.IsDefault() || i.IsHyperV()
}

// DefaultDaemonNetworkMode returns the default network stack the daemon should
// use.
func DefaultDaemonNetworkMode() NetworkMode {
	return NetworkMode("default")
}

// NetworkName returns the name of the network stack.
func (n NetworkMode) NetworkName() string {
	if n.IsDefault() {
		return "default"
	}
	return ""
}

// MergeConfigs merges the specified container Config and HostConfig.
// It creates a ContainerConfigWrapper.
func MergeConfigs(config *Config, hostConfig *HostConfig) *ContainerConfigWrapper {
	return &ContainerConfigWrapper{
		config,
		hostConfig,
	}
}

// IsPreDefinedNetwork indicates if a network is predefined by the daemon
func IsPreDefinedNetwork(network string) bool {
	return false
}

// ValidateNetMode ensures that the various combinations of requested
// network settings are valid.
func ValidateNetMode(c *Config, hc *HostConfig) error {
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

// ValidateIsolationLevel performs platform specific validation of the
// isolation level in the hostconfig structure. Windows supports 'default' (or
// blank), and 'hyperv'. These refer to Windows Server Containers and
// Hyper-V Containers respectively.
func ValidateIsolationLevel(hc *HostConfig) error {
	// We may not be passed a host config, such as in the case of docker commit
	if hc == nil {
		return nil
	}
	if !hc.Isolation.IsValid() {
		return fmt.Errorf("invalid --isolation: %q. Windows supports 'default' (Windows Server Container) or 'hyperv' (Hyper-V Container)", hc.Isolation)
	}
	return nil
}

package runconfig

import "strings"

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

package runconfig

// IsDefault indicates whether container uses the default network stack.
func (n NetworkMode) IsDefault() bool {
	return n == "default"
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

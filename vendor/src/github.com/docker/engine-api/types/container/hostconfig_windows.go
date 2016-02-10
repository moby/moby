package container

import (
	"fmt"
	"strings"
)

// IsDefault indicates whether container uses the default network stack.
func (n NetworkMode) IsDefault() bool {
	return n == "default"
}

// IsNone indicates whether container isn't using a network stack.
func (n NetworkMode) IsNone() bool {
	return n == "none"
}

// IsUserDefined indicates user-created network
func (n NetworkMode) IsUserDefined() bool {
	return !n.IsDefault() && !n.IsNone()
}

// IsHyperV indicates the use of a Hyper-V partition for isolation
func (i Isolation) IsHyperV() bool {
	return strings.ToLower(string(i)) == "hyperv"
}

// IsProcess indicates the use of process isolation
func (i Isolation) IsProcess() bool {
	return strings.ToLower(string(i)) == "process"
}

// IsValid indicates if an isolation technology is valid
func (i Isolation) IsValid() bool {
	return i.IsDefault() || i.IsHyperV() || i.IsProcess()
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

// ValidateIsolationperforms platform specific validation of the
// isolation technology in the hostconfig structure. Windows supports 'default' (or
// blank), 'process', or 'hyperv'.
func ValidateIsolation(hc *HostConfig) error {
	// We may not be passed a host config, such as in the case of docker commit
	if hc == nil {
		return nil
	}
	if !hc.Isolation.IsValid() {
		return fmt.Errorf("invalid --isolation: %q. Windows supports 'default', 'process', or 'hyperv'", hc.Isolation)
	}
	return nil
}

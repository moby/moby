//go:build !windows

package runconfig // import "github.com/docker/docker/runconfig"

import (
	"fmt"
	"runtime"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/sysinfo"
)

// DefaultDaemonNetworkMode returns the default network stack the daemon should
// use.
func DefaultDaemonNetworkMode() container.NetworkMode {
	return "bridge"
}

// IsPreDefinedNetwork indicates if a network is predefined by the daemon
func IsPreDefinedNetwork(network string) bool {
	n := container.NetworkMode(network)
	return n.IsBridge() || n.IsHost() || n.IsNone() || n.IsDefault()
}

// validateNetMode ensures that the various combinations of requested
// network settings are valid.
func validateNetMode(c *container.Config, hc *container.HostConfig) error {
	err := validateNetContainerMode(c, hc)
	if err != nil {
		return err
	}
	if hc.UTSMode.IsHost() && c.Hostname != "" {
		return ErrConflictUTSHostname
	}
	if hc.NetworkMode.IsHost() && len(hc.Links) > 0 {
		return ErrConflictHostNetworkAndLinks
	}
	return nil
}

// validateIsolation performs platform specific validation of
// isolation in the hostconfig structure. Linux only supports "default"
// which is LXC container isolation
func validateIsolation(hc *container.HostConfig) error {
	if !hc.Isolation.IsValid() {
		return fmt.Errorf("Invalid isolation: %q - %s only supports 'default'", hc.Isolation, runtime.GOOS)
	}
	return nil
}

// validateQoS performs platform specific validation of the QoS settings
func validateQoS(hc *container.HostConfig) error {
	if hc.IOMaximumBandwidth != 0 {
		return fmt.Errorf("Invalid QoS settings: %s does not support configuration of maximum bandwidth", runtime.GOOS)
	}
	if hc.IOMaximumIOps != 0 {
		return fmt.Errorf("Invalid QoS settings: %s does not support configuration of maximum IOPs", runtime.GOOS)
	}
	return nil
}

// validateResources performs platform specific validation of the resource settings
// cpu-rt-runtime and cpu-rt-period can not be greater than their parent, cpu-rt-runtime requires sys_nice
func validateResources(hc *container.HostConfig, si *sysinfo.SysInfo) error {
	if (hc.Resources.CPURealtimePeriod != 0 || hc.Resources.CPURealtimeRuntime != 0) && !si.CPURealtime {
		return fmt.Errorf("Your kernel does not support CPU real-time scheduler")
	}
	if hc.Resources.CPURealtimePeriod != 0 && hc.Resources.CPURealtimeRuntime != 0 && hc.Resources.CPURealtimeRuntime > hc.Resources.CPURealtimePeriod {
		return fmt.Errorf("cpu real-time runtime cannot be higher than cpu real-time period")
	}
	return nil
}

// validatePrivileged performs platform specific validation of the Privileged setting
func validatePrivileged(_ *container.HostConfig) error {
	return nil
}

// validateReadonlyRootfs performs platform specific validation of the ReadonlyRootfs setting
func validateReadonlyRootfs(_ *container.HostConfig) error {
	return nil
}

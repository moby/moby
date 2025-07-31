//go:build !windows

package runconfig

import (
	"fmt"
	"runtime"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/pkg/sysinfo"
)

// validateNetMode ensures that the various combinations of requested
// network settings are valid.
func validateNetMode(c *container.Config, hc *container.HostConfig) error {
	err := validateNetContainerMode(c, hc)
	if err != nil {
		return err
	}
	if hc.UTSMode.IsHost() && c.Hostname != "" {
		return validationError("conflicting options: hostname and the UTS mode")
	}
	if hc.NetworkMode.IsHost() && len(hc.Links) > 0 {
		return validationError("conflicting options: host type networking can't be used with links. This would result in undefined behavior")
	}
	return nil
}

// validateIsolation performs platform specific validation of
// isolation in the hostconfig structure. Linux only supports "default"
// which is LXC container isolation
func validateIsolation(hc *container.HostConfig) error {
	if !hc.Isolation.IsValid() {
		return validationError(fmt.Sprintf("invalid isolation (%s): %s only supports 'default'", hc.Isolation, runtime.GOOS))
	}
	return nil
}

// validateQoS performs platform specific validation of the QoS settings
func validateQoS(hc *container.HostConfig) error {
	if hc.IOMaximumBandwidth != 0 {
		return validationError(fmt.Sprintf("invalid option: QoS maximum bandwidth configuration is not supported on %s", runtime.GOOS))
	}
	if hc.IOMaximumIOps != 0 {
		return validationError(fmt.Sprintf("invalid option: QoS maximum IOPs configuration is not supported on %s", runtime.GOOS))
	}
	return nil
}

// validateResources performs platform specific validation of the resource settings
// cpu-rt-runtime and cpu-rt-period can not be greater than their parent, cpu-rt-runtime requires sys_nice
func validateResources(hc *container.HostConfig, si *sysinfo.SysInfo) error {
	if (hc.Resources.CPURealtimePeriod != 0 || hc.Resources.CPURealtimeRuntime != 0) && !si.CPURealtime {
		return validationError("kernel does not support CPU real-time scheduler")
	}
	if hc.Resources.CPURealtimePeriod != 0 && hc.Resources.CPURealtimeRuntime != 0 && hc.Resources.CPURealtimeRuntime > hc.Resources.CPURealtimePeriod {
		return validationError("cpu real-time runtime cannot be higher than cpu real-time period")
	}
	if si.CPUShares {
		// We're only producing an error if CPU-shares are supported to preserve
		// existing behavior. The OCI runtime may still reject the config though.
		// We should consider making this an error-condition when trying to set
		// CPU-shares on a system that doesn't support it instead of silently
		// ignoring.
		if hc.Resources.CPUShares < 0 {
			return validationError(fmt.Sprintf("invalid CPU shares (%d): value must be a positive integer", hc.Resources.CPUShares))
		}
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

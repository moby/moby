package runconfig

import (
	"fmt"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/pkg/sysinfo"
)

// validateNetMode ensures that the various combinations of requested
// network settings are valid.
func validateNetMode(c *container.Config, hc *container.HostConfig) error {
	if err := validateNetContainerMode(c, hc); err != nil {
		return err
	}
	if hc.NetworkMode.IsContainer() && hc.Isolation.IsHyperV() {
		return validationError("invalid network-mode: using the network stack of another container is not supported while using Hyper-V Containers")
	}
	return nil
}

// validateIsolation performs platform specific validation of the
// isolation in the hostconfig structure. Windows supports 'default' (or
// blank), 'process', or 'hyperv'.
func validateIsolation(hc *container.HostConfig) error {
	if !hc.Isolation.IsValid() {
		return validationError(fmt.Sprintf("invalid isolation (%s): Windows supports 'default', 'process', or 'hyperv'", hc.Isolation))
	}
	return nil
}

// validateQoS performs platform specific validation of the Qos settings
func validateQoS(_ *container.HostConfig) error {
	return nil
}

// validateResources performs platform specific validation of the resource settings
func validateResources(hc *container.HostConfig, _ *sysinfo.SysInfo) error {
	if hc.Resources.CPURealtimePeriod != 0 {
		return validationError("invalid option: CPU real-time period is not supported for Windows containers")
	}
	if hc.Resources.CPURealtimeRuntime != 0 {
		return validationError("invalid option: CPU real-time runtime is not supported for Windows containers")
	}
	return nil
}

// validatePrivileged performs platform specific validation of the Privileged setting
func validatePrivileged(hc *container.HostConfig) error {
	if hc.Privileged {
		return validationError("invalid option: privileged mode is not supported for Windows containers")
	}
	return nil
}

// validateReadonlyRootfs performs platform specific validation of the ReadonlyRootfs setting
func validateReadonlyRootfs(hc *container.HostConfig) error {
	if hc.ReadonlyRootfs {
		return validationError("invalid option: read-only mode is not supported for Windows containers")
	}
	return nil
}

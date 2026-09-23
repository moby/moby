package config

import (
	"fmt"

	"github.com/containerd/cgroups/v3"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/daemon/libnetwork/drivers/bridge"
)

func defaultCgroupNamespaceMode() container.CgroupnsMode {
	if cgroups.Mode() != cgroups.Unified {
		return DefaultCgroupV1NamespaceMode
	}
	return DefaultCgroupNamespaceMode
}

func validateFixedCIDRV6(val string) error {
	return bridge.ValidateFixedCIDRV6(val)
}

// validatePlatformExecOpt validates if the given exec-opt and value are valid
// for the current platform.
func validatePlatformExecOpt(opt, value string) error {
	switch opt {
	case "isolation":
		return fmt.Errorf("option '%s' is only supported on windows", opt)
	case "native.cgroupdriver":
		// TODO(thaJeztah): add validation that's currently in daemon.verifyCgroupDriver
		return nil
	default:
		return fmt.Errorf("unknown option: '%s'", opt)
	}
}

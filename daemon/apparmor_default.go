//go:build linux

package daemon

import (
	"fmt"

	"github.com/containerd/containerd/v2/pkg/apparmor"
	"github.com/moby/moby/v2/daemon/internal/rootless"
	aaprofile "github.com/moby/profiles/apparmor"
)

// Define constants for native driver
const (
	unconfinedAppArmorProfile = "unconfined"
	defaultAppArmorProfile    = "docker-default"
)

// DefaultApparmorProfile returns the name of the default apparmor profile
func DefaultApparmorProfile() string {
	if apparmor.HostSupports() {
		if detachedNetNS, _ := rootless.DetachedNetNS(); detachedNetNS != "" {
			// AppArmor is inaccessible with detached-netns because sysfs is netns-scoped.
			return ""
		}
		return defaultAppArmorProfile
	}
	return ""
}

func ensureDefaultAppArmorProfile() error {
	hostSupports := apparmor.HostSupports()
	if hostSupports {
		if detachedNetNS, _ := rootless.DetachedNetNS(); detachedNetNS != "" {
			// "open /sys/kernel/security/apparmor/profiles: permission denied"
			// (because sysfs is netns-scoped)
			hostSupports = false
		}
	}
	if hostSupports {
		loaded, err := aaprofile.IsLoaded(defaultAppArmorProfile)
		if err != nil {
			return fmt.Errorf("Could not check if %s AppArmor profile was loaded: %s", defaultAppArmorProfile, err)
		}

		// Nothing to do.
		if loaded {
			return nil
		}

		// Load the profile.
		if err := aaprofile.InstallDefault(defaultAppArmorProfile); err != nil {
			return fmt.Errorf("AppArmor enabled on system but the %s profile could not be loaded: %s", defaultAppArmorProfile, err)
		}
	}

	return nil
}

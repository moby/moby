//go:build linux

package daemon

import (
	"fmt"

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
	if appArmorSupported() {
		return defaultAppArmorProfile
	}
	return ""
}

func loadDefaultAppArmorProfileIfMissing() error {
	if !defaultAppArmorProfileSupported() {
		return nil
	}

	loaded, err := aaprofile.IsLoaded(defaultAppArmorProfile)
	if err != nil {
		return fmt.Errorf("Could not check if %s AppArmor profile was loaded: %s", defaultAppArmorProfile, err)
	}
	if loaded {
		return nil
	}

	return installDefaultAppArmorProfile()
}

func installDefaultAppArmorProfile() error {
	if defaultAppArmorProfileSupported() {
		if err := aaprofile.InstallDefault(defaultAppArmorProfile); err != nil {
			return fmt.Errorf("AppArmor enabled on system but the %s profile could not be loaded: %s", defaultAppArmorProfile, err)
		}
	}

	return nil
}

func defaultAppArmorProfileSupported() bool {
	hostSupports := appArmorSupported()
	if hostSupports {
		if detachedNetNS, _ := rootless.DetachedNetNS(); detachedNetNS != "" {
			// "open /sys/kernel/security/apparmor/profiles: permission denied"
			// (because sysfs is netns-scoped)
			hostSupports = false
		}
	}

	return hostSupports
}

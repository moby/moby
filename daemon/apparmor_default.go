// +build linux

package daemon // import "github.com/docker/docker/daemon"

import (
	"fmt"

	aaprofile "github.com/docker/docker/profiles/apparmor"
	"github.com/opencontainers/runc/libcontainer/apparmor"
)

// Define constants for native driver
const (
	unconfinedAppArmorProfile = "unconfined"
	defaultAppArmorProfile    = "docker-default"
)

// installDefaultAppArmorProfile installs the configured default daemon
// profile, overwriting any existing profile (if installed).
func (daemon *Daemon) installDefaultAppArmorProfile() error {
	if !apparmor.IsEnabled() {
		return nil
	}

	// Load the profile.
	var err error
	if daemon.appArmorProfile != nil {
		err = aaprofile.InstallCustom(daemon.appArmorProfile, defaultAppArmorProfile)
	} else {
		err = aaprofile.InstallDefault(defaultAppArmorProfile)
	}
	if err != nil {
		return fmt.Errorf("AppArmor enabled on system but the %s profile could not be loaded: %s", defaultAppArmorProfile, err)
	}

	return nil
}

// ensureDefaultAppArmorProfile installs the configured default daemon profile
// (using installDefaultAppArmorProfile) if the profile is not already loaded
// (otherwise it does nothing).
func (daemon *Daemon) ensureDefaultAppArmorProfile() error {
	if !apparmor.IsEnabled() {
		return nil
	}

	loaded, err := aaprofile.IsLoaded(defaultAppArmorProfile)
	if err != nil {
		return fmt.Errorf("Could not check if %s AppArmor profile was loaded: %s", defaultAppArmorProfile, err)
	}

	// Nothing to do.
	if loaded {
		return nil
	}

	// Load the profile.
	return daemon.installDefaultAppArmorProfile()
}

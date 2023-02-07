//go:build linux
// +build linux

package daemon // import "github.com/docker/docker/daemon"

import (
	"fmt"
	"os"
	"sync"

	"github.com/containerd/containerd/pkg/apparmor"
	aaprofile "github.com/docker/docker/profiles/apparmor"
	"github.com/sirupsen/logrus"
)

// Define constants for native driver
const (
	unconfinedAppArmorProfile = "unconfined"
	defaultAppArmorProfile    = "docker-default"
)

var (
	checkAppArmorOnce   sync.Once
	isAppArmorAvailable bool
)

// DefaultApparmorProfile returns the name of the default apparmor profile
func DefaultApparmorProfile() string {
	if apparmor.HostSupports() {
		return defaultAppArmorProfile
	}
	return ""
}

func ensureDefaultAppArmorProfile() error {
	checkAppArmorOnce.Do(func() {
		if apparmor.HostSupports() {
			// Restore the apparmor_parser check removed in containerd:
			// https://github.com/containerd/containerd/commit/1acca8bba36e99684ee3489ea4a42609194ca6b9
			// Fixes: https://github.com/moby/moby/issues/44900
			if _, err := os.Stat("/sbin/apparmor_parser"); err == nil {
				isAppArmorAvailable = true
			} else {
				logrus.Warn("AppArmor enabled on system but \"apparmor_parser\" binary is missing, so profile can't be loaded")
			}
		}
	})

	if isAppArmorAvailable {
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

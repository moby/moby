// +build linux

package daemon

import (
	"github.com/Sirupsen/logrus"
	aaprofile "github.com/docker/docker/profiles/apparmor"
	"github.com/opencontainers/runc/libcontainer/apparmor"
)

// Define constants for native driver
const (
	defaultApparmorProfile = "docker-default"
)

func installDefaultAppArmorProfile() {
	if apparmor.IsEnabled() {
		if err := aaprofile.InstallDefault(defaultApparmorProfile); err != nil {
			apparmorProfiles := []string{defaultApparmorProfile}

			// Allow daemon to run if loading failed, but are active
			// (possibly through another run, manually, or via system startup)
			for _, policy := range apparmorProfiles {
				if err := aaprofile.IsLoaded(policy); err != nil {
					logrus.Errorf("AppArmor enabled on system but the %s profile could not be loaded.", policy)
				}
			}
		}
	}
}

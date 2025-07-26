//go:build !windows

package daemon

import (
	"github.com/moby/moby/v2/daemon/container"
	"github.com/moby/moby/v2/errdefs"
)

func (daemon *Daemon) saveAppArmorConfig(container *container.Container) error {
	container.AppArmorProfile = "" // we don't care about the previous value.

	if !daemon.RawSysInfo().AppArmor {
		return nil // if apparmor is disabled there is nothing to do here.
	}

	if err := parseSecurityOpt(&container.SecurityOptions, container.HostConfig); err != nil {
		return errdefs.InvalidParameter(err)
	}

	if container.HostConfig.Privileged {
		container.AppArmorProfile = unconfinedAppArmorProfile
	} else if container.AppArmorProfile == "" {
		container.AppArmorProfile = defaultAppArmorProfile
	}
	return nil
}

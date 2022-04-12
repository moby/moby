//go:build !windows
// +build !windows

package daemon // import "github.com/docker/docker/daemon"

import (
	"github.com/docker/docker/container"
	"github.com/docker/docker/errdefs"
)

func (daemon *Daemon) saveAppArmorConfig(container *container.Container) error {
	container.AppArmorProfile = "" // we don't care about the previous value.

	if !daemon.apparmorEnabled {
		return nil // if apparmor is disabled there is nothing to do here.
	}

	if err := parseSecurityOpt(container, container.HostConfig); err != nil {
		return errdefs.InvalidParameter(err)
	}

	if !container.HostConfig.Privileged {
		if container.AppArmorProfile == "" {
			container.AppArmorProfile = defaultAppArmorProfile
		}

	} else {
		container.AppArmorProfile = unconfinedAppArmorProfile
	}
	return nil
}

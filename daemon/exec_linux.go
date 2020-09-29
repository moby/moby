package daemon // import "github.com/docker/docker/daemon"

import (
	"context"

	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/exec"
	"github.com/docker/docker/oci/caps"
	"github.com/opencontainers/runc/libcontainer/apparmor"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func (daemon *Daemon) execSetPlatformOpt(c *container.Container, ec *exec.Config, p *specs.Process) error {
	if len(ec.User) > 0 {
		var err error
		p.User, err = getUser(c, ec.User)
		if err != nil {
			return err
		}
	}
	if ec.Privileged {
		if p.Capabilities == nil {
			p.Capabilities = &specs.LinuxCapabilities{}
		}
		p.Capabilities.Bounding = caps.GetAllCapabilities()
		p.Capabilities.Permitted = p.Capabilities.Bounding
		p.Capabilities.Inheritable = p.Capabilities.Bounding
		p.Capabilities.Effective = p.Capabilities.Bounding
	}
	if apparmor.IsEnabled() {
		var appArmorProfile string
		if c.AppArmorProfile != "" {
			appArmorProfile = c.AppArmorProfile
		} else if c.HostConfig.Privileged {
			// `docker exec --privileged` does not currently disable AppArmor
			// profiles. Privileged configuration of the container is inherited
			appArmorProfile = unconfinedAppArmorProfile
		} else {
			appArmorProfile = defaultAppArmorProfile
		}

		if appArmorProfile == defaultAppArmorProfile {
			// Unattended upgrades and other fun services can unload AppArmor
			// profiles inadvertently. Since we cannot store our profile in
			// /etc/apparmor.d, nor can we practically add other ways of
			// telling the system to keep our profile loaded, in order to make
			// sure that we keep the default profile enabled we dynamically
			// reload it if necessary.
			if err := ensureDefaultAppArmorProfile(); err != nil {
				return err
			}
		}
		p.ApparmorProfile = appArmorProfile
	}
	s := &specs.Spec{Process: p}
	return WithRlimits(daemon, c)(context.Background(), nil, nil, s)
}

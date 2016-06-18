// +build linux,seccomp

package daemon

import (
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/container"
	"github.com/docker/docker/profiles/seccomp"
	"github.com/opencontainers/specs/specs-go"
)

func setSeccomp(daemon *Daemon, rs *specs.Spec, c *container.Container) error {
	var profile *specs.Seccomp
	var err error

	if c.HostConfig.Privileged {
		return nil
	}

	if !daemon.seccompEnabled {
		if c.SeccompProfile != "" && c.SeccompProfile != "unconfined" {
			return fmt.Errorf("Seccomp is not enabled in your kernel, cannot run a custom seccomp profile.")
		}
		logrus.Warn("Seccomp is not enabled in your kernel, running container without default profile.")
		c.SeccompProfile = "unconfined"
	}
	if c.SeccompProfile == "unconfined" {
		return nil
	}
	if c.SeccompProfile != "" {
		profile, err = seccomp.LoadProfile(c.SeccompProfile)
		if err != nil {
			return err
		}
	} else {
		profile, err = seccomp.GetDefaultProfile(rs)
		if err != nil {
			return err
		}
	}

	rs.Linux.Seccomp = profile
	return nil
}

package daemon

import (
	"context"
	"errors"

	"github.com/containerd/containerd/v2/core/containers"
	coci "github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/log"
	dconfig "github.com/moby/moby/v2/daemon/config"
	"github.com/moby/moby/v2/daemon/container"
	"github.com/moby/profiles/seccomp"
	"github.com/opencontainers/runtime-spec/specs-go"
)

const supportsSeccomp = true

// WithSeccomp sets the seccomp profile
func WithSeccomp(daemon *Daemon, c *container.Container) coci.SpecOpts {
	return func(ctx context.Context, _ coci.Client, _ *containers.Container, s *coci.Spec) error {
		if c.SeccompProfile == dconfig.SeccompProfileUnconfined {
			return nil
		}
		if c.HostConfig.Privileged {
			var err error
			if c.SeccompProfile != "" {
				s.Linux.Seccomp, err = seccomp.LoadProfile(c.SeccompProfile, s)
			}
			return err
		}
		if !daemon.RawSysInfo().Seccomp {
			if c.SeccompProfile != "" && c.SeccompProfile != dconfig.SeccompProfileDefault {
				return errors.New("seccomp is not enabled in your kernel, cannot run a custom seccomp profile")
			}
			log.G(ctx).Warn("seccomp is not enabled in your kernel, running container without default profile")
			c.SeccompProfile = dconfig.SeccompProfileUnconfined
			return nil
		}
		if s.Linux == nil {
			s.Linux = &specs.Linux{}
		}
		var err error
		switch {
		case c.SeccompProfile == dconfig.SeccompProfileDefault:
			s.Linux.Seccomp, err = seccomp.GetDefaultProfile(s)
		case c.SeccompProfile != "":
			s.Linux.Seccomp, err = seccomp.LoadProfile(c.SeccompProfile, s)
		case daemon.seccompProfile != nil:
			s.Linux.Seccomp, err = seccomp.LoadProfile(string(daemon.seccompProfile), s)
		case daemon.seccompProfilePath == dconfig.SeccompProfileUnconfined:
			c.SeccompProfile = dconfig.SeccompProfileUnconfined
		default:
			s.Linux.Seccomp, err = seccomp.GetDefaultProfile(s)
		}
		return err
	}
}

package daemon // import "github.com/docker/docker/daemon"

import (
	"context"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/oci"
	coci "github.com/containerd/containerd/oci"
	"github.com/containerd/containerd/pkg/apparmor"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/oci/caps"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func withResetAdditionalGIDs() oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, s *oci.Spec) error {
		s.Process.User.AdditionalGids = nil
		return nil
	}
}

func getUserFromContainerd(ctx context.Context, containerdCli *containerd.Client, ec *container.ExecConfig) (specs.User, error) {
	ctr, err := containerdCli.LoadContainer(ctx, ec.Container.ID)
	if err != nil {
		return specs.User{}, err
	}

	cinfo, err := ctr.Info(ctx)
	if err != nil {
		return specs.User{}, err
	}

	spec, err := ctr.Spec(ctx)
	if err != nil {
		return specs.User{}, err
	}

	opts := []oci.SpecOpts{
		coci.WithUser(ec.User),
		withResetAdditionalGIDs(),
		coci.WithAdditionalGIDs(ec.User),
	}
	for _, opt := range opts {
		if err := opt(ctx, containerdCli, &cinfo, spec); err != nil {
			return specs.User{}, err
		}
	}

	return spec.Process.User, nil
}

func (daemon *Daemon) execSetPlatformOpt(ctx context.Context, daemonCfg *config.Config, ec *container.ExecConfig, p *specs.Process) error {
	if len(ec.User) > 0 {
		var err error
		if daemon.UsesSnapshotter() {
			p.User, err = getUserFromContainerd(ctx, daemon.containerdCli, ec)
			if err != nil {
				return err
			}
		} else {
			p.User, err = getUser(ec.Container, ec.User)
			if err != nil {
				return err
			}
		}
	}

	if ec.Privileged {
		p.Capabilities = &specs.LinuxCapabilities{
			Bounding:  caps.GetAllCapabilities(),
			Permitted: caps.GetAllCapabilities(),
			Effective: caps.GetAllCapabilities(),
		}
	}

	if apparmor.HostSupports() {
		var appArmorProfile string
		if ec.Container.AppArmorProfile != "" {
			appArmorProfile = ec.Container.AppArmorProfile
		} else if ec.Container.HostConfig.Privileged {
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
	return withRlimits(daemon, daemonCfg, ec.Container)(ctx, nil, nil, s)
}

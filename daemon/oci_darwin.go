package daemon

import (
	"context"
	"github.com/containerd/containerd/containers"
	coci "github.com/containerd/containerd/oci"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/oci"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

func (daemon *Daemon) mergeUlimits(c *containertypes.HostConfig, daemonCfg *config.Config) {
}

// withCommonOptions sets common docker options
func withCommonOptions(daemon *Daemon, daemonCfg *config.Config, c *container.Container) coci.SpecOpts {
	return func(ctx context.Context, _ coci.Client, _ *containers.Container, s *coci.Spec) error {
		if c.BaseFS == "" {
			return errors.New("populateCommonSpec: BaseFS of container " + c.ID + " is unexpectedly empty")
		}
		linkedEnv, err := daemon.setupLinkedContainers(c)
		if err != nil {
			return err
		}
		s.Root = &specs.Root{
			Path:     c.BaseFS,
			Readonly: c.HostConfig.ReadonlyRootfs,
		}
		if err := c.SetupWorkingDirectory(daemon.idMapping.RootPair()); err != nil {
			return err
		}
		cwd := c.Config.WorkingDir
		if len(cwd) == 0 {
			cwd = "/"
		}
		if s.Process == nil {
			s.Process = &specs.Process{}
		}
		s.Process.Args = append([]string{c.Path}, c.Args...)
		s.Process.Cwd = cwd
		s.Process.Env = c.CreateDaemonEnvironment(c.Config.Tty, linkedEnv)
		s.Process.Terminal = c.Config.Tty

		s.Hostname = c.Config.Hostname

		return nil
	}
}

func (daemon *Daemon) createSpec(ctx context.Context, daemonCfg *configStore, c *container.Container) (retSpec *specs.Spec, err error) {
	var (
		opts []coci.SpecOpts
		s    = oci.DefaultSpec()
	)
	opts = append(opts,
		withCommonOptions(daemon, &daemonCfg.Config, c),
		withMounts(daemon, daemonCfg, c),
		coci.WithAnnotations(c.HostConfig.Annotations),
		WithUser(c),
	)

	if c.NoNewPrivileges {
		opts = append(opts, coci.WithNoNewPrivileges)
	}
	if c.Config.Tty {
		opts = append(opts, WithConsoleSize(c))
	}
	// Set the masked and readonly paths with regard to the host config options if they are set.
	if c.HostConfig.MaskedPaths != nil {
		opts = append(opts, coci.WithMaskedPaths(c.HostConfig.MaskedPaths))
	}
	if c.HostConfig.ReadonlyPaths != nil {
		opts = append(opts, coci.WithReadonlyPaths(c.HostConfig.ReadonlyPaths))
	}

	var snapshotter, snapshotKey string
	if daemon.UsesSnapshotter() {
		snapshotter = daemon.imageService.StorageDriver()
		snapshotKey = c.ID
	}

	return &s, coci.ApplyOpts(ctx, daemon.containerdClient, &containers.Container{
		ID:          c.ID,
		Snapshotter: snapshotter,
		SnapshotKey: snapshotKey,
	}, &s, opts...)
}

// withMounts sets the container's mounts
func withMounts(daemon *Daemon, daemonCfg *configStore, c *container.Container) coci.SpecOpts {
	return func(ctx context.Context, _ coci.Client, _ *containers.Container, s *coci.Spec) (err error) {
		if err := daemon.setupContainerMountsRoot(c); err != nil {
			return err
		}

		if err := daemon.setupIPCDirs(c); err != nil {
			return err
		}

		defer func() {
			if err != nil {
				daemon.cleanupSecretDir(c)
			}
		}()

		if err := daemon.setupSecretDir(c); err != nil {
			return err
		}

		mounts, err := daemon.setupMounts(c)
		if err != nil {
			return err
		}

		for _, m := range mounts {
			s.Mounts = append(s.Mounts, specs.Mount{Destination: m.Destination, Source: m.Source, Type: "bind"})
		}

		return err
	}
}

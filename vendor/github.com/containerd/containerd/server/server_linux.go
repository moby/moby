package server

import (
	"context"
	"os"

	"github.com/containerd/cgroups"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/sys"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// apply sets config settings on the server process
func apply(ctx context.Context, config *Config) error {
	if !config.NoSubreaper {
		log.G(ctx).Info("setting subreaper...")
		if err := sys.SetSubreaper(1); err != nil {
			return err
		}
	}
	if config.OOMScore != 0 {
		log.G(ctx).Debugf("changing OOM score to %d", config.OOMScore)
		if err := sys.SetOOMScore(os.Getpid(), config.OOMScore); err != nil {
			log.G(ctx).WithError(err).Errorf("failed to change OOM score to %d", config.OOMScore)
		}
	}
	if config.Cgroup.Path != "" {
		cg, err := cgroups.Load(cgroups.V1, cgroups.StaticPath(config.Cgroup.Path))
		if err != nil {
			if err != cgroups.ErrCgroupDeleted {
				return err
			}
			if cg, err = cgroups.New(cgroups.V1, cgroups.StaticPath(config.Cgroup.Path), &specs.LinuxResources{}); err != nil {
				return err
			}
		}
		if err := cg.Add(cgroups.Process{
			Pid: os.Getpid(),
		}); err != nil {
			return err
		}
	}
	return nil
}

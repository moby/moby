//go:build !linux && !windows

package daemon

import (
	"context"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/daemon/config"
	"github.com/moby/moby/v2/daemon/container"
	"github.com/moby/moby/v2/errdefs"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func getUser(*container.Container, string) (specs.User, error) {
	return specs.User{}, errdefs.PlatformNotImplemented{Feature: "getUser"}
}

func (daemon *Daemon) mergeUlimits(*containertypes.HostConfig, *config.Config) {}

func (daemon *Daemon) execSetPlatformOpt(context.Context, *config.Config, *container.ExecConfig, *specs.Process) error {
	return nil
}

func (daemon *Daemon) createSpec(context.Context, *configStore, *container.Container, []container.Mount) (*specs.Spec, error) {
	return nil, errdefs.PlatformNotImplemented{Feature: "Daemon.createSpec"}
}

package daemon

import (
	"context"

	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/moby/moby/v2/daemon/config"
	"github.com/moby/moby/v2/daemon/container"
)

func (daemon *Daemon) execSetPlatformOpt(ctx context.Context, daemonCfg *config.Config, ec *container.ExecConfig, p *specs.Process) error {
	if ec.Container.ImagePlatform.OS == "windows" {
		p.User.Username = ec.User
	}
	return nil
}

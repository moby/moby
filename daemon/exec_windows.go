package daemon

import (
	"context"

	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/daemon/container"
)

func (daemon *Daemon) execSetPlatformOpt(ctx context.Context, daemonCfg *config.Config, ec *container.ExecConfig, p *specs.Process) error {
	if ec.Container.ImagePlatform.OS == "windows" {
		p.User.Username = ec.User
	}
	return nil
}

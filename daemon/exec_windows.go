package daemon // import "github.com/docker/docker/daemon"

import (
	"context"

	specs "github.com/opencontainers/runtime-spec/specs-go"

	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/config"
)

func (daemon *Daemon) execSetPlatformOpt(ctx context.Context, daemonCfg *config.Config, ec *container.ExecConfig, p *specs.Process) error {
	if ec.Container.OS == "windows" {
		p.User.Username = ec.User
	}
	return nil
}

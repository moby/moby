package daemon // import "github.com/docker/docker/daemon"

import (
	"context"

	"github.com/docker/docker/container"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func (daemon *Daemon) execSetPlatformOpt(ctx context.Context, ec *container.ExecConfig, p *specs.Process) error {
	if ec.Container.OS == "windows" {
		p.User.Username = ec.User
	}
	return nil
}

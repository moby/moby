package daemon // import "github.com/docker/docker/daemon"

import (
	"context"

	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/exec"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func (daemon *Daemon) execSetPlatformOpt(ctx context.Context, c *container.Container, ec *exec.Config, p *specs.Process) error {
	if c.OS == "windows" {
		p.User.Username = ec.User
	}
	return nil
}

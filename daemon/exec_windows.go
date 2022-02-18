package daemon // import "github.com/moby/moby/daemon"

import (
	"github.com/moby/moby/container"
	"github.com/moby/moby/daemon/exec"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func (daemon *Daemon) execSetPlatformOpt(c *container.Container, ec *exec.Config, p *specs.Process) error {
	if c.OS == "windows" {
		p.User.Username = ec.User
	}
	return nil
}

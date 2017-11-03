package daemon

import (
	"github.com/moby/moby/container"
	"github.com/moby/moby/daemon/exec"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func (daemon *Daemon) execSetPlatformOpt(c *container.Container, ec *exec.Config, p *specs.Process) error {
	// Process arguments need to be escaped before sending to OCI.
	if c.OS == "windows" {
		p.Args = escapeArgs(p.Args)
		p.User.Username = ec.User
	}
	return nil
}

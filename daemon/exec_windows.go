package daemon

import (
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/exec"
	"github.com/docker/docker/libcontainerd"
)

func execSetPlatformOpt(c *container.Container, ec *exec.Config, p *libcontainerd.Process) error {
	// Process arguments need to be escaped before sending to OCI.
	// TODO (jstarks): escape the entrypoint too once the tests are fixed to not rely on this behavior
	p.Args = append([]string{p.Args[0]}, escapeArgs(p.Args[1:])...)
	return nil
}

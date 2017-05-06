package daemon

import (
	"github.com/docker/docker/monolith/container"
	"github.com/docker/docker/monolith/daemon/exec"
	"github.com/docker/docker/monolith/libcontainerd"
)

func execSetPlatformOpt(c *container.Container, ec *exec.Config, p *libcontainerd.Process) error {
	return nil
}

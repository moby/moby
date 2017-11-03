package daemon

import (
	"github.com/moby/moby/container"
	"github.com/moby/moby/daemon/exec"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func (daemon *Daemon) execSetPlatformOpt(_ *container.Container, _ *exec.Config, _ *specs.Process) error {
	return nil
}

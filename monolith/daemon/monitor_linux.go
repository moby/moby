package daemon

import (
	"github.com/docker/docker/monolith/container"
	"github.com/docker/docker/monolith/libcontainerd"
)

// platformConstructExitStatus returns a platform specific exit status structure
func platformConstructExitStatus(e libcontainerd.StateInfo) *container.ExitStatus {
	return &container.ExitStatus{
		ExitCode:  int(e.ExitCode),
		OOMKilled: e.OOMKilled,
	}
}

// postRunProcessing perfoms any processing needed on the container after it has stopped.
func (daemon *Daemon) postRunProcessing(container *container.Container, e libcontainerd.StateInfo) error {
	return nil
}

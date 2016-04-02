package daemon

import (
	"fmt"

	"github.com/docker/docker/container"
	"github.com/docker/docker/libcontainerd"
)

// platformConstructExitStatus returns a platform specific exit status structure
func platformConstructExitStatus(e libcontainerd.StateInfo) *container.ExitStatus {
	return &container.ExitStatus{
		ExitCode: int(e.ExitCode),
	}
}

// postRunProcessing perfoms any processing needed on the container after it has stopped.
func (daemon *Daemon) postRunProcessing(container *container.Container, e libcontainerd.StateInfo) error {
	//TODO Windows - handle update processing here...
	if e.UpdatePending {
		return fmt.Errorf("Windows: Update handling not implemented.")
	}
	return nil
}

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
	if e.ExitCode == 0 && e.UpdatePending {
		spec, err := daemon.createSpec(container)
		if err != nil {
			return err
		}

		servicingOption := &libcontainerd.ServicingOption{
			IsServicing: true,
		}

		// Create a new servicing container, which will start, complete the update, and merge back the
		// results if it succeeded, all as part of the below function call.
		if err := daemon.containerd.Create((container.ID + "_servicing"), *spec, servicingOption); err != nil {
			container.ExitCode = -1
			return fmt.Errorf("Post-run update servicing failed: %s", err)
		}
	}
	return nil
}

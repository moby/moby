package daemon

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker/docker/runconfig"
)

// ContainerCheckpoint checkpoints the process running in a container with CRIU
func (daemon *Daemon) ContainerCheckpoint(name string, opts *runconfig.CriuConfig) error {
	container, err := daemon.GetContainer(name)
	if err != nil {
		return err
	}
	if !container.IsRunning() {
		return fmt.Errorf("Container %s not running", name)
	}

	if opts.ImagesDirectory == "" {
		opts.ImagesDirectory = filepath.Join(container.Root, "criu.image")
		if err := os.MkdirAll(opts.ImagesDirectory, 0755); err != nil && !os.IsExist(err) {
			return err
		}
	}

	if opts.WorkDirectory == "" {
		opts.WorkDirectory = filepath.Join(container.Root, "criu.work")
		if err := os.MkdirAll(opts.WorkDirectory, 0755); err != nil && !os.IsExist(err) {
			return err
		}
	}

	if err := daemon.Checkpoint(container, opts); err != nil {
		return fmt.Errorf("Cannot checkpoint container %s: %s", name, err)
	}

	container.SetCheckpointed(opts.LeaveRunning)
	daemon.LogContainerEvent(container, "checkpoint")

	if opts.LeaveRunning == false {
		daemon.Cleanup(container)
	}

	if err := container.ToDisk(); err != nil {
		return fmt.Errorf("Cannot update config for container: %s", err)
	}

	return nil
}

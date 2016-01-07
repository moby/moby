package daemon

import (
	"fmt"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/container"
)

// ContainerRestore restores the process in a container with CRIU
func (daemon *Daemon) ContainerRestore(name string, opts *types.CriuConfig, forceRestore bool) error {
	container, err := daemon.GetContainer(name)
	if err != nil {
		return err
	}

	if !forceRestore {
		// TODO: It's possible we only want to bypass the checkpointed check,
		// I'm not sure how this will work if the container is already running
		if container.IsRunning() {
			return fmt.Errorf("Container %s already running", name)
		}

		if !container.IsCheckpointed() {
			return fmt.Errorf("Container %s is not checkpointed", name)
		}
	} else {
		if !container.HasBeenCheckpointed() && opts.ImagesDirectory == "" {
			return fmt.Errorf("You must specify an image directory to restore from %s", name)
		}
	}

	if opts.ImagesDirectory == "" {
		opts.ImagesDirectory = filepath.Join(container.Root, "criu.image")
	}

	if opts.WorkDirectory == "" {
		opts.WorkDirectory = filepath.Join(container.Root, "criu.work")
	}

	if err = daemon.containerRestore(container, opts, forceRestore); err != nil {
		return fmt.Errorf("Cannot restore container %s: %s", name, err)
	}

	return nil
}

// containerRestore prepares the container to be restored by setting up
// everything the container needs, just like containerStart, such as
// storage and networking, as well as links between containers.
// The container is left waiting for a signal that restore has finished
func (daemon *Daemon) containerRestore(container *container.Container, opts *types.CriuConfig, forceRestore bool) error {
	return daemon.containerStartOrRestore(container, opts, forceRestore)
}

func (daemon *Daemon) waitForRestore(container *container.Container, opts *types.CriuConfig, forceRestore bool) error {
	return container.RestoreMonitor(daemon, container.HostConfig.RestartPolicy, opts, forceRestore)
}

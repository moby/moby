package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"fmt"

	"github.com/containerd/containerd/log"
	"github.com/docker/docker/container"
)

// ContainerPause pauses a container
func (daemon *Daemon) ContainerPause(name string) error {
	ctr, err := daemon.GetContainer(name)
	if err != nil {
		return err
	}
	return daemon.containerPause(ctr)
}

// containerPause pauses the container execution without stopping the process.
// The execution can be resumed by calling containerUnpause.
func (daemon *Daemon) containerPause(container *container.Container) error {
	container.Lock()
	defer container.Unlock()

	// We cannot Pause the container which is not running
	tsk, err := container.GetRunningTask()
	if err != nil {
		return err
	}

	// We cannot Pause the container which is already paused
	if container.Paused {
		return errNotPaused(container.ID)
	}

	// We cannot Pause the container which is restarting
	if container.Restarting {
		return errContainerIsRestarting(container.ID)
	}

	if err := tsk.Pause(context.Background()); err != nil {
		return fmt.Errorf("cannot pause container %s: %s", container.ID, err)
	}

	container.Paused = true
	daemon.setStateCounter(container)
	daemon.updateHealthMonitor(container)
	daemon.LogContainerEvent(container, "pause")

	if err := container.CheckpointTo(daemon.containersReplica); err != nil {
		log.G(context.TODO()).WithError(err).Warn("could not save container to disk")
	}

	return nil
}

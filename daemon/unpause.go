package daemon // import "github.com/moby/moby/daemon"

import (
	"context"
	"fmt"

	"github.com/moby/moby/container"
	"github.com/sirupsen/logrus"
)

// ContainerUnpause unpauses a container
func (daemon *Daemon) ContainerUnpause(name string) error {
	ctr, err := daemon.GetContainer(name)
	if err != nil {
		return err
	}
	return daemon.containerUnpause(ctr)
}

// containerUnpause resumes the container execution after the container is paused.
func (daemon *Daemon) containerUnpause(ctr *container.Container) error {
	ctr.Lock()
	defer ctr.Unlock()

	// We cannot unpause the container which is not paused
	if !ctr.Paused {
		return fmt.Errorf("Container %s is not paused", ctr.ID)
	}

	if err := daemon.containerd.Resume(context.Background(), ctr.ID); err != nil {
		return fmt.Errorf("Cannot unpause container %s: %s", ctr.ID, err)
	}

	ctr.Paused = false
	daemon.setStateCounter(ctr)
	daemon.updateHealthMonitor(ctr)
	daemon.LogContainerEvent(ctr, "unpause")

	if err := ctr.CheckpointTo(daemon.containersReplica); err != nil {
		logrus.WithError(err).Warn("could not save container to disk")
	}

	return nil
}

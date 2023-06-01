package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"fmt"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/container"
)

// ContainerRestart stops and starts a container. It attempts to
// gracefully stop the container within the given timeout, forcefully
// stopping it if the timeout is exceeded. If given a negative
// timeout, ContainerRestart will wait forever until a graceful
// stop. Returns an error if the container cannot be found, or if
// there is an underlying error at any stage of the restart.
func (daemon *Daemon) ContainerRestart(ctx context.Context, name string, options containertypes.StopOptions) error {
	ctr, err := daemon.GetContainer(name)
	if err != nil {
		return err
	}
	err = daemon.containerRestart(ctx, daemon.config(), ctr, options)
	if err != nil {
		return fmt.Errorf("Cannot restart container %s: %v", name, err)
	}
	return nil
}

// containerRestart attempts to gracefully stop and then start the
// container. When stopping, wait for the given duration in seconds to
// gracefully stop, before forcefully terminating the container. If
// given a negative duration, wait forever for a graceful stop.
func (daemon *Daemon) containerRestart(ctx context.Context, daemonCfg *configStore, container *container.Container, options containertypes.StopOptions) error {
	// Determine isolation. If not specified in the hostconfig, use daemon default.
	actualIsolation := container.HostConfig.Isolation
	if containertypes.Isolation.IsDefault(actualIsolation) {
		actualIsolation = daemon.defaultIsolation
	}

	// Avoid unnecessarily unmounting and then directly mounting
	// the container when the container stops and then starts
	// again. We do not do this for Hyper-V isolated containers
	// (implying also on Windows) as the HCS must have exclusive
	// access to mount the containers filesystem inside the utility
	// VM.
	if !containertypes.Isolation.IsHyperV(actualIsolation) {
		if err := daemon.Mount(container); err == nil {
			defer daemon.Unmount(container)
		}
	}

	if container.IsRunning() {
		container.Lock()
		container.HasBeenManuallyRestarted = true
		container.Unlock()

		err := daemon.containerStop(ctx, container, options)

		if err != nil {
			return err
		}
	}

	if err := daemon.containerStart(ctx, daemonCfg, container, "", "", true); err != nil {
		return err
	}

	daemon.LogContainerEvent(container, "restart")
	return nil
}

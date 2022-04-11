package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"fmt"

	libcontainerdtypes "github.com/docker/docker/libcontainerd/types"
)

// ContainerResize changes the size of the TTY of the process running
// in the container with the given name to the given height and width.
func (daemon *Daemon) ContainerResize(ctx context.Context, name string, height, width int) error {
	container, err := daemon.GetContainer(ctx, name)
	if err != nil {
		return err
	}

	if !container.IsRunning() {
		return errNotRunning(container.ID)
	}

	if err = daemon.containerd.ResizeTerminal(ctx, container.ID, libcontainerdtypes.InitProcessName, width, height); err == nil {
		attributes := map[string]string{
			"height": fmt.Sprintf("%d", height),
			"width":  fmt.Sprintf("%d", width),
		}
		daemon.LogContainerEventWithAttributes(container, "resize", attributes)
	}
	return err
}

// ContainerExecResize changes the size of the TTY of the process
// running in the exec with the given name to the given height and
// width.
func (daemon *Daemon) ContainerExecResize(ctx context.Context, name string, height, width int) error {
	ec, err := daemon.getExecConfig(name)
	if err != nil {
		return err
	}

	select {
	case <-ec.Started:
		return daemon.containerd.ResizeTerminal(ctx, ec.ContainerID, ec.ID, width, height)
	case <-ctx.Done():
		return ctx.Err()
	}
}

package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/errdefs"
)

// ContainerResize changes the size of the TTY of the process running
// in the container with the given name to the given height and width.
func (daemon *Daemon) ContainerResize(ctx context.Context, name string, height, width uint32) error {
	container, err := daemon.GetContainer(name)
	if err != nil {
		return err
	}

	container.Lock()
	tsk, err := container.GetRunningTask()
	container.Unlock()
	if err != nil {
		return err
	}

	if err = tsk.Resize(context.WithoutCancel(ctx), width, height); err == nil {
		daemon.LogContainerEventWithAttributes(container, events.ActionResize, map[string]string{
			"height": strconv.FormatUint(uint64(height), 10),
			"width":  strconv.FormatUint(uint64(width), 10),
		})
	}
	return err
}

// ContainerExecResize changes the size of the TTY of the process
// running in the exec with the given name to the given height and
// width.
func (daemon *Daemon) ContainerExecResize(ctx context.Context, name string, height, width uint32) error {
	ec, err := daemon.getExecConfig(name)
	if err != nil {
		return err
	}

	// TODO: the timeout is hardcoded here, it would be more flexible to make it
	// a parameter in resize request context, which would need API changes.
	timeout := time.NewTimer(10 * time.Second)
	defer timeout.Stop()

	select {
	case <-ec.Started:
		// An error may have occurred, so ec.Process may be nil.
		if ec.Process == nil {
			return errdefs.InvalidParameter(errors.New("exec process is not started"))
		}
		return ec.Process.Resize(context.WithoutCancel(ctx), width, height)
	case <-timeout.C:
		return errors.New("timeout waiting for exec session ready")
	}
}

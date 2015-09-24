package daemon

import (
	"time"

	"github.com/docker/docker/context"
)

// ContainerWait stops processing until the given container is
// stopped. If the container is not found, an error is returned. On a
// successful stop, the exit code of the container is returned. On a
// timeout, an error is returned. If you want to wait forever, supply
// a negative duration for the timeout.
func (daemon *Daemon) ContainerWait(ctx context.Context, name string, timeout time.Duration) (int, error) {
	container, err := daemon.Get(ctx, name)
	if err != nil {
		return -1, err
	}

	return container.WaitStop(timeout)
}

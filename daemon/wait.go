package daemon

import (
	"github.com/docker/docker/container"
	"golang.org/x/net/context"
)

// ContainerWait stops processing until the given container is stopped or
// removed (if untilRemoved is true). If the container is not found, a nil
// channel and non-nil error is returned immediately. If the container is
// found, a status result will be sent on the returned channel once the wait
// condition is met or if an error occurs waiting for the container (such as a
// context timeout or cancellation). On a successful stop, the exit code of the
// container is returned in the status with a non-nil Err() value.
func (daemon *Daemon) ContainerWait(ctx context.Context, name string, untilRemoved bool) (<-chan *container.StateStatus, error) {
	container, err := daemon.GetContainer(name)
	if err != nil {
		return nil, err
	}

	return container.Wait(ctx, untilRemoved), nil
}

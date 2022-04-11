package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"errors"
	"time"

	"github.com/docker/docker/pkg/archive"
)

// ContainerChanges returns a list of container fs changes
func (daemon *Daemon) ContainerChanges(ctx context.Context, name string) ([]archive.Change, error) {
	start := time.Now()
	container, err := daemon.GetContainer(ctx, name)
	if err != nil {
		return nil, err
	}

	if isWindows && container.IsRunning() {
		return nil, errors.New("Windows does not support diff of a running container")
	}

	container.Lock()
	defer container.Unlock()
	if container.RWLayer == nil {
		return nil, errors.New("RWLayer of container " + name + " is unexpectedly nil")
	}
	c, err := container.RWLayer.Changes()
	if err != nil {
		return nil, err
	}
	containerActions.WithValues("changes").UpdateSince(start)
	return c, nil
}

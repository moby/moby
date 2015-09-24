package daemon

import (
	"github.com/docker/docker/context"
	"github.com/docker/docker/pkg/archive"
)

// ContainerChanges returns a list of container fs changes
func (daemon *Daemon) ContainerChanges(ctx context.Context, name string) ([]archive.Change, error) {
	container, err := daemon.Get(ctx, name)
	if err != nil {
		return nil, err
	}

	return container.changes()
}

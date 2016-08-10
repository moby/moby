package daemon

import "github.com/docker/docker/pkg/archive"

// ContainerChanges returns a list of container fs changes
func (daemon *Daemon) ContainerChanges(name string) ([]archive.Change, error) {
	container, err := daemon.GetContainer(name)
	if err != nil {
		return nil, err
	}

	// make sure the container isn't removed while we are working
	daemon.opLock.Lock(container.ID)
	defer daemon.opLock.Unlock(container.ID)
	return container.RWLayer.Changes()
}

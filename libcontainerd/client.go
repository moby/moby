package libcontainerd

import (
	"fmt"
	"sync"

	"github.com/docker/docker/pkg/locker"
)

type containerNotFoundError struct {
	containerID string
}

func (e containerNotFoundError) Error() string {
	return fmt.Sprintf("invalid container: %s", e.containerID)
}

func (containerNotFoundError) NotFound() {}

// clientCommon contains the platform agnostic fields used in the client structure
type clientCommon struct {
	backend    Backend
	containers map[string]*container
	locker     *locker.Locker
	mapMutex   sync.RWMutex // protects read/write operations from containers map
}

func (clnt *client) lock(containerID string) {
	clnt.locker.Lock(containerID)
}

func (clnt *client) unlock(containerID string) {
	clnt.locker.Unlock(containerID)
}

// must hold a lock for cont.containerID
func (clnt *client) appendContainer(cont *container) {
	clnt.mapMutex.Lock()
	clnt.containers[cont.containerID] = cont
	clnt.mapMutex.Unlock()
}
func (clnt *client) deleteContainer(containerID string) {
	clnt.mapMutex.Lock()
	delete(clnt.containers, containerID)
	clnt.mapMutex.Unlock()
}

func (clnt *client) getContainer(containerID string) (*container, error) {
	clnt.mapMutex.RLock()
	container, ok := clnt.containers[containerID]
	defer clnt.mapMutex.RUnlock()
	if !ok {
		return nil, &containerNotFoundError{
			containerID: containerID,
		}
	}
	return container, nil
}

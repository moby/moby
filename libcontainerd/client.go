package libcontainerd

import (
	"fmt"
	"sync"

	"github.com/Sirupsen/logrus"
)

// clientCommon contains the platform agnostic fields used in the client structure
type clientCommon struct {
	backend          Backend
	containers       map[string]*container
	containerMutexes map[string]*sync.Mutex // lock by container ID
	mapMutex         sync.RWMutex           // protects read/write oprations from containers map
	sync.Mutex                              // lock for containerMutexes map access
}

func (clnt *client) lock(containerID string) {
	clnt.Lock()
	if _, ok := clnt.containerMutexes[containerID]; !ok {
		clnt.containerMutexes[containerID] = &sync.Mutex{}
	}
	clnt.Unlock()
	clnt.containerMutexes[containerID].Lock()
}

func (clnt *client) unlock(containerID string) {
	clnt.Lock()
	if l, ok := clnt.containerMutexes[containerID]; ok {
		l.Unlock()
	} else {
		logrus.Warnf("unlock of non-existing mutex: %s", containerID)
	}
	clnt.Unlock()
}

// must hold a lock for cont.containerID
func (clnt *client) appendContainer(cont *container) {
	clnt.mapMutex.Lock()
	clnt.containers[cont.containerID] = cont
	clnt.mapMutex.Unlock()
}
func (clnt *client) deleteContainer(friendlyName string) {
	clnt.mapMutex.Lock()
	delete(clnt.containers, friendlyName)
	clnt.mapMutex.Unlock()
}

func (clnt *client) getContainer(containerID string) (*container, error) {
	clnt.mapMutex.RLock()
	container, ok := clnt.containers[containerID]
	defer clnt.mapMutex.RUnlock()
	if !ok {
		return nil, fmt.Errorf("invalid container: %s", containerID) // fixme: typed error
	}
	return container, nil
}

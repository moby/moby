package container

import (
	"sync"

	"github.com/docker/docker/pkg/locker"
)

// memoryStore implements a Store in memory.
// It uses two locks to keep concurrent
// accesss control to the containers.
// - globalLock controls all accesses to the
// map store. This lock MUST always be acquired
// first in order to avoid deadlocks.
// - namedLock controls accesses to specific
// container IDs. This prevents accessing
// references that could have been garbage
// collected, like inspecting a container that
// was removed concurrently.
type memoryStore struct {
	s          map[string]*Container
	globalLock sync.Mutex
	namedLock  *locker.Locker
}

// NewMemoryStore initializes a new memory store.
func NewMemoryStore() Store {
	return &memoryStore{
		s:         make(map[string]*Container),
		namedLock: locker.New(),
	}
}

// Add appends a new container to the memory store.
// It overrides the id if it existed before.
func (c *memoryStore) Add(id string, cont *Container) {
	c.globalLock.Lock()
	c.s[id] = cont
	c.globalLock.Unlock()
}

// Get returns a container from the store by id.
func (c *memoryStore) Get(id string) *Container {
	c.globalLock.Lock()
	c.namedLock.Lock(id)
	res := c.s[id]
	c.namedLock.Unlock(id)
	c.globalLock.Unlock()
	return res
}

// Delete removes a container from the store by id.
func (c *memoryStore) Delete(id string) {
	c.globalLock.Lock()
	c.namedLock.Lock(id)
	delete(c.s, id)
	c.namedLock.Unlock(id)
	c.globalLock.Unlock()
}

// List returns a sorted list of containers from the store.
// The containers are ordered by creation date.
func (c *memoryStore) List() []*Container {
	containers := new(History)
	c.globalLock.Lock()
	for _, cont := range c.s {
		containers.Add(cont)
	}
	c.globalLock.Unlock()
	containers.sort()
	return *containers
}

// Size returns the number of containers in the store.
func (c *memoryStore) Size() int {
	c.globalLock.Lock()
	l := len(c.s)
	c.globalLock.Unlock()
	return l
}

// First returns the first container found in the store by a given filter.
func (c *memoryStore) First(filter StoreFilter) *Container {
	c.globalLock.Lock()
	defer c.globalLock.Unlock()
	for _, cont := range c.s {
		if filter(cont) {
			return cont
		}
	}
	return nil
}

// ApplyAll calls the reducer function with every container in the store.
// This operation is asyncronous in the memory store.
func (c *memoryStore) ApplyAll(apply StoreReducer) {
	c.globalLock.Lock()
	defer c.globalLock.Unlock()

	wg := new(sync.WaitGroup)
	for _, cont := range c.s {
		wg.Add(1)
		c.namedLock.Lock(cont.ID)
		go func(container *Container) {
			defer func() {
				c.namedLock.Unlock(container.ID)
				wg.Done()
			}()
			apply(container)
		}(cont)
	}

	wg.Wait()
}

// ReduceAll filters a list of containers and calls the reducer function with each one of them.
func (c *memoryStore) ReduceAll(filter StoreFilter, apply StoreReducer) error {
	var containers []*Container
	c.globalLock.Lock()
	for _, cont := range c.s {
		if filter(cont) {
			containers = append(containers, cont)
		}
	}
	c.globalLock.Unlock()

	for _, cont := range containers {
		c.namedLock.Lock(cont.ID)
		if err := apply(cont); err != nil {
			c.namedLock.Unlock(cont.ID)
			return err
		}
		c.namedLock.Unlock(cont.ID)
	}
	return nil
}

// ReduceOne gets a controller and calls the reducer function with it.
func (c *memoryStore) ReduceOne(id string, apply StoreReducer) error {
	c.namedLock.Lock(id)
	c.globalLock.Lock()
	defer func() {
		c.globalLock.Unlock()
		c.namedLock.Unlock(id)
	}()
	cont := c.s[id]
	if cont == nil {
		return nil
	}
	return apply(cont)
}

var _ Store = &memoryStore{}

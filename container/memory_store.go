package container // import "github.com/docker/docker/container"

import (
	"sync"
)

// MemoryStore implements a Store in memory.
type MemoryStore struct {
	s  map[string]*Container
	mu sync.RWMutex
}

// NewMemoryStore initializes a new memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		s: make(map[string]*Container),
	}
}

// Add appends a new container to the memory store.
// It overrides the id if it existed before.
func (c *MemoryStore) Add(id string, cont *Container) {
	c.mu.Lock()
	c.s[id] = cont
	c.mu.Unlock()
}

// Get returns a container from the store by id.
func (c *MemoryStore) Get(id string) *Container {
	var res *Container
	c.mu.RLock()
	res = c.s[id]
	c.mu.RUnlock()
	return res
}

// Delete removes a container from the store by id.
func (c *MemoryStore) Delete(id string) {
	c.mu.Lock()
	delete(c.s, id)
	c.mu.Unlock()
}

// List returns a sorted list of containers from the store.
// The containers are ordered by creation date.
func (c *MemoryStore) List() []*Container {
	containers := History(c.all())
	containers.sort()
	return containers
}

// Size returns the number of containers in the store.
func (c *MemoryStore) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.s)
}

// First returns the first container found in the store by a given filter.
func (c *MemoryStore) First(filter StoreFilter) *Container {
	for _, cont := range c.all() {
		if filter(cont) {
			return cont
		}
	}
	return nil
}

// ApplyAll calls the reducer function with every container in the store.
// This operation is asynchronous in the memory store.
// NOTE: Modifications to the store MUST NOT be done by the StoreReducer.
func (c *MemoryStore) ApplyAll(apply StoreReducer) {
	wg := new(sync.WaitGroup)
	for _, cont := range c.all() {
		wg.Add(1)
		go func(container *Container) {
			apply(container)
			wg.Done()
		}(cont)
	}

	wg.Wait()
}

func (c *MemoryStore) all() []*Container {
	c.mu.RLock()
	containers := make([]*Container, 0, len(c.s))
	for _, cont := range c.s {
		containers = append(containers, cont)
	}
	c.mu.RUnlock()
	return containers
}

package container

import "sync"

// memoryStore implements a Store in memory.
type memoryStore struct {
	s map[string]*Container
	sync.Mutex
}

// NewMemoryStore initializes a new memory store.
func NewMemoryStore() Store {
	return &memoryStore{
		s: make(map[string]*Container),
	}
}

// Add appends a new container to the memory store.
// It overrides the id if it existed before.
func (c *memoryStore) Add(id string, cont *Container) {
	c.Lock()
	c.s[id] = cont
	c.Unlock()
}

// Get returns a container from the store by id.
func (c *memoryStore) Get(id string) *Container {
	c.Lock()
	res := c.s[id]
	c.Unlock()
	return res
}

// Delete removes a container from the store by id.
func (c *memoryStore) Delete(id string) {
	c.Lock()
	delete(c.s, id)
	c.Unlock()
}

// List returns a sorted list of containers from the store.
// The containers are ordered by creation date.
func (c *memoryStore) List() []*Container {
	containers := new(History)
	c.Lock()
	for _, cont := range c.s {
		containers.Add(cont)
	}
	c.Unlock()
	containers.sort()
	return *containers
}

// Size returns the number of containers in the store.
func (c *memoryStore) Size() int {
	c.Lock()
	defer c.Unlock()
	return len(c.s)
}

// First returns the first container found in the store by a given filter.
func (c *memoryStore) First(filter StoreFilter) *Container {
	c.Lock()
	defer c.Unlock()
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
	c.Lock()
	defer c.Unlock()

	wg := new(sync.WaitGroup)
	for _, cont := range c.s {
		wg.Add(1)
		go func(container *Container) {
			apply(container)
			wg.Done()
		}(cont)
	}

	wg.Wait()
}

var _ Store = &memoryStore{}

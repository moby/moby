package container

import (
	"reflect"
	"sync"
)

// memoryStore implements a Store in memory.
type memoryStore struct {
	s    map[string]*Container
	lock sync.Mutex
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
	c.lock.Lock()
	c.s[id] = cont
	c.lock.Unlock()
}

// Get returns a container from the store by id.
func (c *memoryStore) Get(id string) *Container {
	c.lock.Lock()
	res := c.s[id]
	c.lock.Unlock()
	return res
}

// Delete removes a container from the store by id.
func (c *memoryStore) Delete(id string) {
	c.lock.Lock()
	delete(c.s, id)
	c.lock.Unlock()
}

// List returns a sorted list of containers from the store.
// The containers are ordered by creation date.
func (c *memoryStore) List() []*Container {
	containers := new(History)
	c.lock.Lock()
	for _, cont := range c.s {
		containers.Add(cont)
	}
	c.lock.Unlock()
	containers.sort()
	return *containers
}

// Size returns the number of containers in the store.
func (c *memoryStore) Size() int {
	c.lock.Lock()
	l := len(c.s)
	c.lock.Unlock()
	return l
}

// First returns the first container found in the store by a given filter.
func (c *memoryStore) First(filter StoreFilter) *Container {
	c.lock.Lock()
	defer c.lock.Unlock()
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
	c.lock.Lock()
	defer c.lock.Unlock()

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

// ReduceAll filters a list of containers and calls the reducer function with each one of them.
func (c *memoryStore) ReduceAll(filter StoreFilter, apply StoreReducer) error {
	var containers []*Container
	c.lock.Lock()
	for _, cont := range c.s {
		if filter(cont) {
			cp := copyContainer(cont)
			containers = append(containers, cp)
		}
	}
	c.lock.Unlock()

	for _, cont := range containers {
		if err := apply(cont); err != nil {
			return err
		}
	}
	return nil
}

// ReduceOne gets a controller and calls the reducer function with it.
func (c *memoryStore) ReduceOne(id string, apply StoreReducer) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	cont := c.s[id]
	if cont == nil {
		return nil
	}
	return apply(copyContainer(cont))
}

var _ Store = &memoryStore{}

// copyContainer recursively deep copies a container pointer.
func copyContainer(cont *Container) *Container {
	original := reflect.ValueOf(cont)
	cp := reflect.New(original.Type()).Elem()

	copyRecursive(original, cp)
	copied := cp.Interface().(*Container)

	return copied
}

// copyRecursive iterates recursively over the structure and
// copies its values.
func copyRecursive(original, cp reflect.Value) {
	// handle according to original's Kind
	switch original.Kind() {
	case reflect.Ptr:
		// Get the actual value being pointed to.
		originalValue := original.Elem()
		// if  it isn't valid, return.
		if !originalValue.IsValid() {
			return
		}
		cp.Set(reflect.New(originalValue.Type()))
		copyRecursive(originalValue, cp.Elem())
	case reflect.Interface:
		// Get the value for the interface, not the pointer.
		originalValue := original.Elem()
		if !originalValue.IsValid() {
			return
		}
		// Get the value by calling Elem().
		copyValue := reflect.New(originalValue.Type()).Elem()
		copyRecursive(originalValue, copyValue)
		cp.Set(copyValue)
	case reflect.Struct:
		// Go through each field of the struct and copy it.
		var exported bool
		for i := 0; i < original.NumField(); i++ {
			if cp.Field(i).CanSet() {
				copyRecursive(original.Field(i), cp.Field(i))
				exported = true
			}
		}
		// Copy the complete struct if none of the fields were exported.
		// This means it's a stdlib struct, like time.Time.
		if !exported {
			cp.Set(original)
		}
	case reflect.Slice:
		// Make a new slice and copy each element.
		cp.Set(reflect.MakeSlice(original.Type(), original.Len(), original.Cap()))
		for i := 0; i < original.Len(); i++ {
			copyRecursive(original.Index(i), cp.Index(i))
		}
	case reflect.Map:
		cp.Set(reflect.MakeMap(original.Type()))
		for _, key := range original.MapKeys() {
			originalValue := original.MapIndex(key)
			copyValue := reflect.New(originalValue.Type()).Elem()
			copyRecursive(originalValue, copyValue)
			cp.SetMapIndex(key, copyValue)
		}
	// Set the actual values from here on.
	case reflect.String:
		cp.SetString(original.String())
	case reflect.Int:
		cp.SetInt(original.Int())
	case reflect.Bool:
		cp.SetBool(original.Bool())
	case reflect.Float64:
		cp.SetFloat(original.Float())
	default:
		cp.Set(original)
	}
}

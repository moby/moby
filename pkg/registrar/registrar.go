// Package registrar provides name registration. It reserves a name to a given key.
package registrar

import (
	"errors"
	"sync"
)

var (
	// ErrNameReserved is an error which is returned when a name is requested to be reserved that already is reserved
	ErrNameReserved = errors.New("name is reserved")
	// ErrNameNotReserved is an error which is returned when trying to find a name that is not reserved
	ErrNameNotReserved = errors.New("name is not reserved")
	// ErrNoSuchKey is returned when trying to find the names for a key which is not known
	ErrNoSuchKey = errors.New("provided key does not exist")
)

// Registrar stores indexes a list of keys and their registered names as well as indexes names and the key that they are registered to
// Names must be unique.
// Registrar is safe for concurrent access.
type Registrar struct {
	idx   map[string][]string
	names map[string]string
	mu    sync.Mutex
}

// NewRegistrar creates a new Registrar with the an empty index
func NewRegistrar() *Registrar {
	return &Registrar{
		idx:   make(map[string][]string),
		names: make(map[string]string),
	}
}

// Reserve registers a key to a name
// Reserve is idempotent
// Attempting to reserve a key to a name that already exists results in an `ErrNameReserved`
// A name reservation is globally unique
func (r *Registrar) Reserve(name, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if k, exists := r.names[name]; exists {
		if k != key {
			return ErrNameReserved
		}
		return nil
	}

	r.idx[key] = append(r.idx[key], name)
	r.names[name] = key
	return nil
}

// Release releases the reserved name
// Once released, a name can be reserved again
func (r *Registrar) Release(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key, exists := r.names[name]
	if !exists {
		return
	}

	for i, n := range r.idx[key] {
		if n != name {
			continue
		}
		r.idx[key] = append(r.idx[key][:i], r.idx[key][i+1:]...)
		break
	}

	delete(r.names, name)

	if len(r.idx[key]) == 0 {
		delete(r.idx, key)
	}
}

// Delete removes all reservations for the passed in key.
// All names reserved to this key are released.
func (r *Registrar) Delete(key string) {
	r.mu.Lock()
	for _, name := range r.idx[key] {
		delete(r.names, name)
	}
	delete(r.idx, key)
	r.mu.Unlock()
}

// GetNames lists all the reserved names for the given key
func (r *Registrar) GetNames(key string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	names, exists := r.idx[key]
	if !exists {
		return nil, ErrNoSuchKey
	}
	return names, nil
}

// Get returns the key that the passed in name is reserved to
func (r *Registrar) Get(name string) (string, error) {
	r.mu.Lock()
	key, exists := r.names[name]
	r.mu.Unlock()

	if !exists {
		return "", ErrNameNotReserved
	}
	return key, nil
}

// GetAll returns all registered names
func (r *Registrar) GetAll() map[string][]string {
	out := make(map[string][]string)

	r.mu.Lock()
	// copy index into out
	for id, names := range r.idx {
		out[id] = names
	}
	r.mu.Unlock()
	return out
}

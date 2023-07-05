// Package idm manages reservation/release of numerical ids from a configured set of contiguous ids
package idm

import (
	"errors"
	"fmt"
	"sync"

	"github.com/docker/docker/libnetwork/bitmap"
)

// Idm manages the reservation/release of numerical ids from a contiguous set
type Idm struct {
	start  uint64
	end    uint64
	mu     sync.Mutex
	handle *bitmap.Bitmap
}

// placeholder is a type for function arguments which need to be present for Swarmkit
// to compile, but for which the only acceptable value is nil.
type placeholder *struct{}

// New returns an instance of id manager for a [start,end] set of numerical ids
func New(_ placeholder, _ string, start, end uint64) (*Idm, error) {
	if end <= start {
		return nil, fmt.Errorf("invalid set range: [%d, %d]", start, end)
	}

	return &Idm{start: start, end: end, handle: bitmap.New(1 + end - start)}, nil
}

// GetID returns the first available id in the set
func (i *Idm) GetID(serial bool) (uint64, error) {
	if i.handle == nil {
		return 0, errors.New("ID set is not initialized")
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	ordinal, err := i.handle.SetAny(serial)
	return i.start + ordinal, err
}

// GetSpecificID tries to reserve the specified id
func (i *Idm) GetSpecificID(id uint64) error {
	if i.handle == nil {
		return errors.New("ID set is not initialized")
	}

	if id < i.start || id > i.end {
		return errors.New("Requested id does not belong to the set")
	}

	i.mu.Lock()
	defer i.mu.Unlock()
	return i.handle.Set(id - i.start)
}

// GetIDInRange returns the first available id in the set within a [start,end] range
func (i *Idm) GetIDInRange(start, end uint64, serial bool) (uint64, error) {
	if i.handle == nil {
		return 0, errors.New("ID set is not initialized")
	}

	if start < i.start || end > i.end {
		return 0, errors.New("Requested range does not belong to the set")
	}

	i.mu.Lock()
	defer i.mu.Unlock()
	ordinal, err := i.handle.SetAnyInRange(start-i.start, end-i.start, serial)

	return i.start + ordinal, err
}

// Release releases the specified id
func (i *Idm) Release(id uint64) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.handle.Unset(id - i.start)
}

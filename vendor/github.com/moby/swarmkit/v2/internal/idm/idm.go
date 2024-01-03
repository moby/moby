// Package idm manages reservation/release of numerical ids from a configured set of contiguous ids.
package idm

import (
	"errors"
	"fmt"

	"github.com/docker/docker/libnetwork/bitmap"
)

// IDM manages the reservation/release of numerical ids from a contiguous set.
//
// An IDM instance is not safe for concurrent use.
type IDM struct {
	start  uint64
	end    uint64
	handle *bitmap.Bitmap
}

// New returns an instance of id manager for a [start,end] set of numerical ids.
func New(start, end uint64) (*IDM, error) {
	if end <= start {
		return nil, fmt.Errorf("invalid set range: [%d, %d]", start, end)
	}

	return &IDM{start: start, end: end, handle: bitmap.New(1 + end - start)}, nil
}

// GetID returns the first available id in the set.
func (i *IDM) GetID(serial bool) (uint64, error) {
	if i.handle == nil {
		return 0, errors.New("ID set is not initialized")
	}
	ordinal, err := i.handle.SetAny(serial)
	return i.start + ordinal, err
}

// GetSpecificID tries to reserve the specified id.
func (i *IDM) GetSpecificID(id uint64) error {
	if i.handle == nil {
		return errors.New("ID set is not initialized")
	}

	if id < i.start || id > i.end {
		return errors.New("requested id does not belong to the set")
	}

	return i.handle.Set(id - i.start)
}

// GetIDInRange returns the first available id in the set within a [start,end] range.
func (i *IDM) GetIDInRange(start, end uint64, serial bool) (uint64, error) {
	if i.handle == nil {
		return 0, errors.New("ID set is not initialized")
	}

	if start < i.start || end > i.end {
		return 0, errors.New("requested range does not belong to the set")
	}

	ordinal, err := i.handle.SetAnyInRange(start-i.start, end-i.start, serial)

	return i.start + ordinal, err
}

// Release releases the specified id.
func (i *IDM) Release(id uint64) {
	i.handle.Unset(id - i.start)
}

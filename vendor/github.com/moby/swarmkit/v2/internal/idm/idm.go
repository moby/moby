// Package idm manages reservation/release of numerical ids from a configured set of contiguous ids.
package idm

import (
	"errors"
	"fmt"

	"github.com/bits-and-blooms/bitset"
)

var (
	// ErrNoBitAvailable is returned when no more bits are available to set
	ErrNoBitAvailable = errors.New("no bit available")
	// ErrBitAllocated is returned when the specific bit requested is already set
	ErrBitAllocated = errors.New("requested bit is already allocated")
)

// IDM manages the reservation/release of numerical ids from a contiguous set.
//
// An IDM instance is not safe for concurrent use.
type IDM struct {
	start, end uint
	set        *bitset.BitSet
	next       uint // index of the bit to start searching for the next serial allocation from (not offset by start)
}

// New returns an instance of id manager for a [start,end] set of numerical ids.
func New(start, end uint) (*IDM, error) {
	if end <= start {
		return nil, fmt.Errorf("invalid set range: [%d, %d]", start, end)
	}

	return &IDM{start: start, end: end, set: bitset.New(1 + end - start)}, nil
}

// GetID returns the first available id in the set.
func (i *IDM) GetID(serial bool) (uint, error) {
	if i.set == nil {
		return 0, errors.New("ID set is not initialized")
	}
	var (
		ordinal uint
		ok      bool
	)
	if serial && i.next != 0 {
		ordinal, ok = i.set.NextClear(i.next)
		if ok {
			goto found
		}
	}
	ordinal, ok = i.set.NextClear(0)
	if !ok {
		return 0, ErrNoBitAvailable
	}

found:
	i.set.Set(ordinal)
	i.next = ordinal + 1
	if i.next > i.end-i.start {
		i.next = 0
	}
	return i.start + ordinal, nil
}

// GetSpecificID tries to reserve the specified id.
func (i *IDM) GetSpecificID(id uint) error {
	if i.set == nil {
		return errors.New("ID set is not initialized")
	}

	if id < i.start || id > i.end {
		return errors.New("requested id does not belong to the set")
	}
	if i.set.Test(id - i.start) {
		return ErrBitAllocated
	}
	i.set.Set(id - i.start)
	return nil
}

// Release releases the specified id.
func (i *IDM) Release(id uint) {
	i.set.Clear(id - i.start)
}

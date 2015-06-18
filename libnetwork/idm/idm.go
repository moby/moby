// Package idm manages resevation/release of numerical ids from a configured set of contiguos ids
package idm

import (
	"fmt"

	"github.com/docker/libnetwork/bitseq"
	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/types"
)

// Idm manages the reservation/release of numerical ids from a contiguos set
type Idm struct {
	start  uint32
	end    uint32
	handle *bitseq.Handle
}

// New returns an instance of id manager for a set of [start-end] numerical ids
func New(ds datastore.DataStore, id string, start, end uint32) (*Idm, error) {
	if id == "" {
		return nil, fmt.Errorf("Invalid id")
	}
	if end <= start {
		return nil, fmt.Errorf("Invalid set range: [%d, %d]", start, end)
	}

	h, err := bitseq.NewHandle("idm", ds, id, uint32(1+end-start))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize bit sequence handler: %s", err.Error())
	}

	return &Idm{start: start, end: end, handle: h}, nil
}

// GetID returns the first available id in the set
func (i *Idm) GetID() (uint32, error) {
	if i.handle == nil {
		return 0, fmt.Errorf("ID set is not initialized")
	}

	for {
		bytePos, bitPos, err := i.handle.GetFirstAvailable()
		if err != nil {
			return 0, fmt.Errorf("no available ids")
		}
		id := i.start + uint32(bitPos+bytePos*8)

		// for sets which length is non multiple of 32 this check is needed
		if i.end < id {
			return 0, fmt.Errorf("no available ids")
		}

		if err := i.handle.PushReservation(bytePos, bitPos, false); err != nil {
			if _, ok := err.(types.RetryError); !ok {
				return 0, fmt.Errorf("internal failure while reserving the id: %s", err.Error())
			}
			continue
		}

		return id, nil
	}
}

// GetSpecificID tries to reserve the specified id
func (i *Idm) GetSpecificID(id uint32) error {
	if i.handle == nil {
		return fmt.Errorf("ID set is not initialized")
	}

	if id < i.start || id > i.end {
		return fmt.Errorf("Requested id does not belong to the set")
	}

	for {
		bytePos, bitPos, err := i.handle.CheckIfAvailable(int(id - i.start))
		if err != nil {
			return fmt.Errorf("requested id is not available")
		}
		if err := i.handle.PushReservation(bytePos, bitPos, false); err != nil {
			if _, ok := err.(types.RetryError); !ok {
				return fmt.Errorf("internal failure while reserving the id: %s", err.Error())
			}
			continue
		}
		return nil
	}
}

// Release releases the specified id
func (i *Idm) Release(id uint32) {
	ordinal := id - i.start
	i.handle.PushReservation(int(ordinal/8), int(ordinal%8), true)
}

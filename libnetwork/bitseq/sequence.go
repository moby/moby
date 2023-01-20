// Package bitseq provides a structure and utilities for representing a long
// bitmask which is persisted in a datastore. It is backed by [bitmap.Bitmap]
// which operates directly on the encoded representation, without uncompressing.
package bitseq

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/docker/docker/libnetwork/bitmap"
	"github.com/docker/docker/libnetwork/datastore"
	"github.com/docker/docker/libnetwork/types"
	"github.com/sirupsen/logrus"
)

var (
	// ErrNoBitAvailable is returned when no more bits are available to set
	ErrNoBitAvailable = bitmap.ErrNoBitAvailable
	// ErrBitAllocated is returned when the specific bit requested is already set
	ErrBitAllocated = bitmap.ErrBitAllocated
)

// Handle contains the sequence representing the bitmask and its identifier
type Handle struct {
	app      string
	id       string
	dbIndex  uint64
	dbExists bool
	store    datastore.DataStore
	bm       *bitmap.Bitmap
	mu       sync.Mutex
}

// NewHandle returns a thread-safe instance of the bitmask handler
func NewHandle(app string, ds datastore.DataStore, id string, numElements uint64) (*Handle, error) {
	h := &Handle{
		bm:    bitmap.New(numElements),
		app:   app,
		id:    id,
		store: ds,
	}

	if h.store == nil {
		return h, nil
	}

	// Get the initial status from the ds if present.
	if err := h.store.GetObject(datastore.Key(h.Key()...), h); err != nil && err != datastore.ErrKeyNotFound {
		return nil, err
	}

	// If the handle is not in store, write it.
	if !h.Exists() {
		if err := h.writeToStore(); err != nil {
			return nil, fmt.Errorf("failed to write bitsequence to store: %v", err)
		}
	}

	return h, nil
}

func (h *Handle) getCopy() *Handle {
	return &Handle{
		bm:       bitmap.Copy(h.bm),
		app:      h.app,
		id:       h.id,
		dbIndex:  h.dbIndex,
		dbExists: h.dbExists,
		store:    h.store,
	}
}

// SetAnyInRange atomically sets the first unset bit in the specified range in the sequence and returns the corresponding ordinal
func (h *Handle) SetAnyInRange(start, end uint64, serial bool) (uint64, error) {
	return h.apply(func(b *bitmap.Bitmap) (uint64, error) { return b.SetAnyInRange(start, end, serial) })
}

// SetAny atomically sets the first unset bit in the sequence and returns the corresponding ordinal
func (h *Handle) SetAny(serial bool) (uint64, error) {
	return h.apply(func(b *bitmap.Bitmap) (uint64, error) { return b.SetAny(serial) })
}

// Set atomically sets the corresponding bit in the sequence
func (h *Handle) Set(ordinal uint64) error {
	_, err := h.apply(func(b *bitmap.Bitmap) (uint64, error) { return 0, b.Set(ordinal) })
	return err
}

// Unset atomically unsets the corresponding bit in the sequence
func (h *Handle) Unset(ordinal uint64) error {
	_, err := h.apply(func(b *bitmap.Bitmap) (uint64, error) { return 0, b.Unset(ordinal) })
	return err
}

// IsSet atomically checks if the ordinal bit is set. In case ordinal
// is outside of the bit sequence limits, false is returned.
func (h *Handle) IsSet(ordinal uint64) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.bm.IsSet(ordinal)
}

// CheckConsistency checks if the bit sequence is in an inconsistent state and attempts to fix it.
// It looks for a corruption signature that may happen in docker 1.9.0 and 1.9.1.
func (h *Handle) CheckConsistency() error {
	for {
		h.mu.Lock()
		store := h.store
		h.mu.Unlock()

		if store != nil {
			if err := store.GetObject(datastore.Key(h.Key()...), h); err != nil && err != datastore.ErrKeyNotFound {
				return err
			}
		}

		h.mu.Lock()
		nh := h.getCopy()
		h.mu.Unlock()

		if !nh.bm.CheckConsistency() {
			return nil
		}

		if err := nh.writeToStore(); err != nil {
			if _, ok := err.(types.RetryError); !ok {
				return fmt.Errorf("internal failure while fixing inconsistent bitsequence: %v", err)
			}
			continue
		}

		logrus.Infof("Fixed inconsistent bit sequence in datastore:\n%s\n%s", h, nh)

		h.mu.Lock()
		h.bm = nh.bm
		h.mu.Unlock()

		return nil
	}
}

// set/reset the bit
func (h *Handle) apply(op func(*bitmap.Bitmap) (uint64, error)) (uint64, error) {
	for {
		var store datastore.DataStore
		h.mu.Lock()
		store = h.store
		if store != nil {
			h.mu.Unlock() // The lock is acquired in the GetObject
			if err := store.GetObject(datastore.Key(h.Key()...), h); err != nil && err != datastore.ErrKeyNotFound {
				return 0, err
			}
			h.mu.Lock() // Acquire the lock back
		}

		// Create a private copy of h and work on it
		nh := h.getCopy()

		ret, err := op(nh.bm)
		if err != nil {
			h.mu.Unlock()
			return ret, err
		}

		if h.store != nil {
			h.mu.Unlock()
			// Attempt to write private copy to store
			if err := nh.writeToStore(); err != nil {
				if _, ok := err.(types.RetryError); !ok {
					return ret, fmt.Errorf("internal failure while setting the bit: %v", err)
				}
				// Retry
				continue
			}
			h.mu.Lock()
		}

		// Previous atomic push was successful. Save private copy to local copy
		h.bm = nh.bm
		h.dbExists = nh.dbExists
		h.dbIndex = nh.dbIndex
		h.mu.Unlock()
		return ret, nil
	}
}

// Destroy removes from the datastore the data belonging to this handle
func (h *Handle) Destroy() error {
	for {
		if err := h.deleteFromStore(); err != nil {
			if _, ok := err.(types.RetryError); !ok {
				return fmt.Errorf("internal failure while destroying the sequence: %v", err)
			}
			// Fetch latest
			if err := h.store.GetObject(datastore.Key(h.Key()...), h); err != nil {
				if err == datastore.ErrKeyNotFound { // already removed
					return nil
				}
				return fmt.Errorf("failed to fetch from store when destroying the sequence: %v", err)
			}
			continue
		}
		return nil
	}
}

// Bits returns the length of the bit sequence
func (h *Handle) Bits() uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.bm.Bits()
}

// Unselected returns the number of bits which are not selected
func (h *Handle) Unselected() uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.bm.Unselected()
}

func (h *Handle) String() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return fmt.Sprintf("App: %s, ID: %s, DBIndex: 0x%x, %s",
		h.app, h.id, h.dbIndex, h.bm)
}

// MarshalJSON encodes h into a JSON message.
func (h *Handle) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"id": h.id,
	}

	b, err := func() ([]byte, error) {
		h.mu.Lock()
		defer h.mu.Unlock()
		return h.bm.MarshalBinary()
	}()
	if err != nil {
		return nil, err
	}
	m["sequence"] = b
	return json.Marshal(m)
}

// UnmarshalJSON decodes a JSON message into h.
func (h *Handle) UnmarshalJSON(data []byte) error {
	var (
		m   map[string]interface{}
		b   []byte
		err error
	)
	if err = json.Unmarshal(data, &m); err != nil {
		return err
	}
	h.id = m["id"].(string)
	bi, _ := json.Marshal(m["sequence"])
	if err := json.Unmarshal(bi, &b); err != nil {
		return err
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.bm.UnmarshalBinary(b); err != nil {
		return err
	}
	return nil
}

package datastore

import (
	"errors"
	"time"

	"github.com/docker/swarm/pkg/store"
)

var (
	// ErrNotImplmented exported
	ErrNotImplmented = errors.New("Functionality not implemented")
)

// MockData exported
type MockData struct {
	Data  []byte
	Index uint64
}

// MockStore exported
type MockStore struct {
	db map[string]*MockData
}

// NewMockStore creates a Map backed Datastore that is useful for mocking
func NewMockStore() *MockStore {
	db := make(map[string]*MockData)
	return &MockStore{db}
}

// Get the value at "key", returns the last modified index
// to use in conjunction to CAS calls
func (s *MockStore) Get(key string) (value []byte, lastIndex uint64, err error) {
	mData := s.db[key]
	if mData == nil {
		return nil, 0, nil
	}
	return mData.Data, mData.Index, nil

}

// Put a value at "key"
func (s *MockStore) Put(key string, value []byte) error {
	mData := s.db[key]
	if mData == nil {
		mData = &MockData{value, 0}
	}
	mData.Index = mData.Index + 1
	s.db[key] = mData
	return nil
}

// Delete a value at "key"
func (s *MockStore) Delete(key string) error {
	delete(s.db, key)
	return nil
}

// Exists checks that the key exists inside the store
func (s *MockStore) Exists(key string) (bool, error) {
	_, ok := s.db[key]
	return ok, nil
}

// GetRange gets a range of values at "directory"
func (s *MockStore) GetRange(prefix string) (values []store.KVEntry, err error) {
	return nil, ErrNotImplmented
}

// DeleteRange deletes a range of values at "directory"
func (s *MockStore) DeleteRange(prefix string) error {
	return ErrNotImplmented
}

// Watch a single key for modifications
func (s *MockStore) Watch(key string, heartbeat time.Duration, callback store.WatchCallback) error {
	return ErrNotImplmented
}

// CancelWatch cancels a watch, sends a signal to the appropriate
// stop channel
func (s *MockStore) CancelWatch(key string) error {
	return ErrNotImplmented
}

// Internal function to check if a key has changed
func (s *MockStore) waitForChange(key string) <-chan uint64 {
	return nil
}

// WatchRange triggers a watch on a range of values at "directory"
func (s *MockStore) WatchRange(prefix string, filter string, heartbeat time.Duration, callback store.WatchCallback) error {
	return ErrNotImplmented
}

// CancelWatchRange stops the watch on the range of values, sends
// a signal to the appropriate stop channel
func (s *MockStore) CancelWatchRange(prefix string) error {
	return ErrNotImplmented
}

// Acquire the lock for "key"/"directory"
func (s *MockStore) Acquire(key string, value []byte) (string, error) {
	return "", ErrNotImplmented
}

// Release the lock for "key"/"directory"
func (s *MockStore) Release(id string) error {
	return ErrNotImplmented
}

// AtomicPut put a value at "key" if the key has not been
// modified in the meantime, throws an error if this is the case
func (s *MockStore) AtomicPut(key string, _ []byte, newValue []byte, index uint64) (bool, error) {
	mData := s.db[key]
	if mData != nil && mData.Index != index {
		return false, errInvalidAtomicRequest
	}
	return true, s.Put(key, newValue)
}

// AtomicDelete deletes a value at "key" if the key has not
// been modified in the meantime, throws an error if this is the case
func (s *MockStore) AtomicDelete(key string, oldValue []byte, index uint64) (bool, error) {
	mData := s.db[key]
	if mData != nil && mData.Index != index {
		return false, errInvalidAtomicRequest
	}
	return true, s.Delete(key)
}

// Close closes the client connection
func (s *MockStore) Close() {
	return
}

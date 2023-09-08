package datastore

import (
	"errors"

	store "github.com/docker/docker/libnetwork/internal/kvstore"
	"github.com/docker/docker/libnetwork/types"
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
	return &MockStore{db: make(map[string]*MockData)}
}

// Get the value at "key", returns the last modified index
// to use in conjunction to CAS calls
func (s *MockStore) Get(key string) (*store.KVPair, error) {
	mData := s.db[key]
	if mData == nil {
		return nil, nil
	}
	return &store.KVPair{Value: mData.Data, LastIndex: mData.Index}, nil
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

// Exists checks that the key exists inside the store
func (s *MockStore) Exists(key string) (bool, error) {
	_, ok := s.db[key]
	return ok, nil
}

// List gets a range of values at "directory"
func (s *MockStore) List(prefix string) ([]*store.KVPair, error) {
	return nil, errors.New("not implemented")
}

// AtomicPut put a value at "key" if the key has not been
// modified in the meantime, throws an error if this is the case
func (s *MockStore) AtomicPut(key string, newValue []byte, previous *store.KVPair) (*store.KVPair, error) {
	mData := s.db[key]

	if previous == nil {
		if mData != nil {
			return nil, types.InvalidParameterErrorf("atomic put failed because key exists")
		} // Else OK.
	} else {
		if mData == nil {
			return nil, types.InvalidParameterErrorf("atomic put failed because key exists")
		}
		if mData != nil && mData.Index != previous.LastIndex {
			return nil, types.InvalidParameterErrorf("atomic put failed due to mismatched Index")
		} // Else OK.
	}
	if err := s.Put(key, newValue); err != nil {
		return nil, err
	}
	return &store.KVPair{Key: key, Value: newValue, LastIndex: s.db[key].Index}, nil
}

// AtomicDelete deletes a value at "key" if the key has not
// been modified in the meantime, throws an error if this is the case
func (s *MockStore) AtomicDelete(key string, previous *store.KVPair) error {
	mData := s.db[key]
	if mData != nil && mData.Index != previous.LastIndex {
		return types.InvalidParameterErrorf("atomic delete failed due to mismatched Index")
	}
	delete(s.db, key)
	return nil
}

// Close closes the client connection
func (s *MockStore) Close() {
}

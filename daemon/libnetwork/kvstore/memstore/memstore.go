// Package memstore provides an in-memory [store.Store] that accepts and
// rejects the same operations as the boltdb package's BoltDB, with the same
// errors.
package memstore

import (
	"bytes"
	"slices"
	"strings"
	"sync"

	store "github.com/moby/moby/v2/daemon/libnetwork/kvstore"
	bolt "go.etcd.io/bbolt"
	berrors "go.etcd.io/bbolt/errors"
)

// metadataLen is the length of the index that BoltDB stores in front of each
// value.
const metadataLen = 8

type entry struct {
	value []byte
	index uint64
}

// MemStore is an in-memory [store.Store].
type MemStore struct {
	mu      sync.Mutex
	entries map[string]entry
	dbIndex uint64
}

var _ store.Store = (*MemStore)(nil)

// New returns an empty MemStore.
func New() *MemStore {
	return &MemStore{entries: map[string]entry{}}
}

// put stores value at key with the next index of the store, and returns that
// index. Like BoltDB, it uses up an index even if it rejects the key or value.
// The caller must hold m.mu.
func (m *MemStore) put(key string, value []byte) (uint64, error) {
	m.dbIndex++
	switch {
	case key == "":
		return 0, berrors.ErrKeyRequired
	case len(key) > bolt.MaxKeySize:
		return 0, berrors.ErrKeyTooLarge
	case int64(metadataLen+len(value)) > bolt.MaxValueSize:
		return 0, berrors.ErrValueTooLarge
	}
	m.entries[key] = entry{value: bytes.Clone(value), index: m.dbIndex}
	return m.dbIndex, nil
}

func (m *MemStore) Put(key string, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, err := m.put(key, value)
	return err
}

func (m *MemStore) Exists(key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.entries[key]; !ok {
		return false, store.ErrKeyNotFound
	}
	return true, nil
}

func (m *MemStore) List(keyPrefix string) ([]*store.KVPair, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var kv []*store.KVPair
	for key, e := range m.entries {
		if strings.HasPrefix(key, keyPrefix) {
			// Like BoltDB, return a non-nil copy even of a nil value.
			val := make([]byte, len(e.value))
			copy(val, e.value)
			kv = append(kv, &store.KVPair{Key: key, Value: val, LastIndex: e.index})
		}
	}
	if len(kv) == 0 {
		return nil, store.ErrKeyNotFound
	}
	slices.SortFunc(kv, func(a, b *store.KVPair) int { return strings.Compare(a.Key, b.Key) })
	return kv, nil
}

func (m *MemStore) AtomicPut(key string, value []byte, previous *store.KVPair) (*store.KVPair, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	e, ok := m.entries[key]
	if previous == nil && ok {
		return nil, store.ErrKeyExists
	}
	if previous != nil {
		if !ok {
			return nil, store.ErrKeyNotFound
		}
		if e.index != previous.LastIndex {
			return nil, store.ErrKeyModified
		}
	}
	index, err := m.put(key, value)
	if err != nil {
		return nil, err
	}
	return &store.KVPair{Key: key, Value: value, LastIndex: index}, nil
}

func (m *MemStore) AtomicDelete(key string, previous *store.KVPair) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if previous == nil {
		return store.ErrPreviousNotSpecified
	}
	e, ok := m.entries[key]
	if !ok {
		return store.ErrKeyNotFound
	}
	if e.index != previous.LastIndex {
		return store.ErrKeyModified
	}
	delete(m.entries, key)
	return nil
}

func (m *MemStore) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.entries[key]; !ok {
		return store.ErrKeyNotFound
	}
	delete(m.entries, key)
	return nil
}

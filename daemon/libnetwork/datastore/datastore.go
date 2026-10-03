package datastore

import (
	"errors"
	"strings"
	"sync"

	store "github.com/moby/moby/v2/daemon/libnetwork/kvstore"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
)

// ErrKeyModified is raised for an atomic update when the update is working on a stale state
var (
	ErrKeyModified = store.ErrKeyModified
	ErrKeyNotFound = store.ErrKeyNotFound
)

// Store keeps KVObjects in memory, and writes them through to a [store.Store],
// except those whose Skip method returns true.
type Store struct {
	mu    sync.Mutex
	store store.Store
	cache *cache
}

// KVObject is Key/Value interface used by objects to be part of the Store.
type KVObject interface {
	// Key method lets an object provide the Key to be used in KV Store
	Key() []string
	// KeyPrefix method lets an object return immediate parent key that can be used for tree walk
	KeyPrefix() []string
	// Value method lets an object marshal its content to be stored in the KV store
	Value() []byte
	// SetValue is used by the datastore to set the object's value when loaded from the data store.
	SetValue([]byte) error
	// Index method returns the latest DB Index as seen by the object
	Index() uint64
	// SetIndex method allows the datastore to store the latest DB Index into the object
	SetIndex(uint64)
	// Exists returns true if the object exists in the datastore, false if it hasn't been stored yet.
	// When SetIndex() is called, the object has been stored.
	Exists() bool
	// Skip provides a way for a KV Object to avoid persisting it in the KV Store
	Skip() bool
	// New returns a new object which is created based on the
	// source object
	New() KVObject
	// CopyTo deep copies the contents of the implementing object
	// to the passed destination object
	CopyTo(KVObject) error
}

const (
	// NetworkKeyPrefix is the prefix for network key in the kv store
	NetworkKeyPrefix = "network"
	// EndpointKeyPrefix is the prefix for endpoint key in the kv store
	EndpointKeyPrefix = "endpoint"
)

var (
	defaultRootChain = []string{"docker", "network", "v1.0"}
	rootChain        = defaultRootChain
)

// Key provides convenient method to create a Key
func Key(key ...string) string {
	var b strings.Builder
	for _, parts := range [][]string{rootChain, key} {
		for _, part := range parts {
			b.WriteString(part)
			b.WriteString("/")
		}
	}
	return b.String()
}

// New returns a Store that writes its objects through to kv.
func New(kv store.Store) *Store {
	return &Store{store: kv, cache: newCache(kv)}
}

// PutObjectAtomic provides an atomic add and update operation for a Record.
func (ds *Store) PutObjectAtomic(kvObject KVObject) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if kvObject == nil {
		return types.InvalidParameterErrorf("invalid KV Object: nil")
	}

	kvObjValue := kvObject.Value()

	if kvObjValue == nil {
		return types.InvalidParameterErrorf("invalid KV Object with a nil Value for key %s", Key(kvObject.Key()...))
	}

	if !kvObject.Skip() {
		var previous *store.KVPair
		if kvObject.Exists() {
			previous = &store.KVPair{Key: Key(kvObject.Key()...), LastIndex: kvObject.Index()}
		}

		pair, err := ds.store.AtomicPut(Key(kvObject.Key()...), kvObjValue, previous)
		if err != nil {
			if errors.Is(err, store.ErrKeyExists) {
				return ErrKeyModified
			}
			return err
		}

		kvObject.SetIndex(pair.LastIndex)
	}

	// If persistent store is skipped, sequencing needs to
	// happen in cache.
	return ds.cache.add(kvObject, kvObject.Skip())
}

// GetObject gets data from the store and unmarshals to the specified object.
func (ds *Store) GetObject(o KVObject) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	return ds.cache.get(o)
}

// List returns of a list of KVObjects belonging to the parent key. The caller
// must pass a KVObject of the same type as the objects that need to be listed.
func (ds *Store) List(kvObject KVObject) ([]KVObject, error) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	return ds.cache.list(kvObject)
}

// DeleteObject deletes a kvObject from the on-disk DB and the in-memory cache.
// Unlike DeleteObjectAtomic, it doesn't check the optimistic lock of the
// passed kvObject.
func (ds *Store) DeleteObject(kvObject KVObject) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if kvObject == nil {
		return types.InvalidParameterErrorf("invalid KV Object: nil")
	}

	if !kvObject.Skip() {
		if err := ds.store.Delete(Key(kvObject.Key()...)); err != nil {
			return err
		}
	}

	// cleanup the cache only if AtomicDelete went through successfully
	// If persistent store is skipped, sequencing needs to
	// happen in cache.
	return ds.cache.del(kvObject, false)
}

// DeleteObjectAtomic performs atomic delete on a record.
func (ds *Store) DeleteObjectAtomic(kvObject KVObject) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if kvObject == nil {
		return types.InvalidParameterErrorf("invalid KV Object: nil")
	}

	previous := &store.KVPair{Key: Key(kvObject.Key()...), LastIndex: kvObject.Index()}

	if !kvObject.Skip() {
		if err := ds.store.AtomicDelete(Key(kvObject.Key()...), previous); err != nil {
			if errors.Is(err, store.ErrKeyExists) {
				return ErrKeyModified
			}
			return err
		}
	}

	// cleanup the cache only if AtomicDelete went through successfully
	// If persistent store is skipped, sequencing needs to
	// happen in cache.
	return ds.cache.del(kvObject, kvObject.Skip())
}

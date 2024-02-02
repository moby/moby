package datastore

import (
	"fmt"
	"strings"
	"sync"
	"time"

	store "github.com/docker/docker/libnetwork/internal/kvstore"
	"github.com/docker/docker/libnetwork/internal/kvstore/boltdb"
	"github.com/docker/docker/libnetwork/types"
)

// ErrKeyModified is raised for an atomic update when the update is working on a stale state
var (
	ErrKeyModified = store.ErrKeyModified
	ErrKeyNotFound = store.ErrKeyNotFound
)

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

// ScopeCfg represents Datastore configuration.
type ScopeCfg struct {
	Client ScopeClientCfg
}

// ScopeClientCfg represents Datastore Client-only mode configuration
type ScopeClientCfg struct {
	Provider string
	Address  string
	Config   *store.Config
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

const defaultPrefix = "/var/lib/docker/network/files"

// DefaultScope returns a default scope config for clients to use.
func DefaultScope(dataDir string) ScopeCfg {
	var dbpath string
	if dataDir == "" {
		dbpath = defaultPrefix + "/local-kv.db"
	} else {
		dbpath = dataDir + "/network/files/local-kv.db"
	}

	return ScopeCfg{
		Client: ScopeClientCfg{
			Provider: string(store.BOLTDB),
			Address:  dbpath,
			Config: &store.Config{
				Bucket:            "libnetwork",
				ConnectionTimeout: time.Minute,
				PersistConnection: true,
			},
		},
	}
}

// IsValid checks if the scope config has valid configuration.
func (cfg *ScopeCfg) IsValid() bool {
	if cfg == nil || strings.TrimSpace(cfg.Client.Provider) == "" || strings.TrimSpace(cfg.Client.Address) == "" {
		return false
	}

	return true
}

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

// newClient used to connect to KV Store
func newClient(kv string, addr string, config *store.Config) (*Store, error) {
	if kv != string(store.BOLTDB) {
		return nil, fmt.Errorf("unsupported KV store")
	}

	if config == nil {
		config = &store.Config{}
	}

	s, err := boltdb.New(addr, config)
	if err != nil {
		return nil, err
	}

	return &Store{store: s, cache: newCache(s)}, nil
}

// New creates a new Store instance.
func New(cfg ScopeCfg) (*Store, error) {
	if cfg.Client.Provider == "" || cfg.Client.Address == "" {
		cfg = DefaultScope("")
	}

	return newClient(cfg.Client.Provider, cfg.Client.Address, cfg.Client.Config)
}

// Close closes the data store.
func (ds *Store) Close() {
	ds.store.Close()
}

// PutObjectAtomic provides an atomic add and update operation for a Record.
func (ds *Store) PutObjectAtomic(kvObject KVObject) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if kvObject == nil {
		return types.InvalidParameterErrorf("invalid KV Object : nil")
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
			if err == store.ErrKeyExists {
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

func (ds *Store) ensureParent(parent string) error {
	exists, err := ds.store.Exists(parent)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return ds.store.Put(parent, []byte{})
}

// List returns of a list of KVObjects belonging to the parent key. The caller
// must pass a KVObject of the same type as the objects that need to be listed.
func (ds *Store) List(kvObject KVObject) ([]KVObject, error) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	return ds.cache.list(kvObject)
}

func (ds *Store) iterateKVPairsFromStore(key string, ctor KVObject, callback func(string, KVObject)) error {
	// Make sure the parent key exists
	if err := ds.ensureParent(key); err != nil {
		return err
	}

	kvList, err := ds.store.List(key)
	if err != nil {
		return err
	}

	for _, kvPair := range kvList {
		if len(kvPair.Value) == 0 {
			continue
		}

		dstO := ctor.New()
		if err := dstO.SetValue(kvPair.Value); err != nil {
			return err
		}

		// Make sure the object has a correct view of the DB index in
		// case we need to modify it and update the DB.
		dstO.SetIndex(kvPair.LastIndex)
		callback(kvPair.Key, dstO)
	}

	return nil
}

// Map returns a Map of KVObjects.
func (ds *Store) Map(key string, kvObject KVObject) (map[string]KVObject, error) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	results := map[string]KVObject{}
	err := ds.iterateKVPairsFromStore(key, kvObject, func(key string, val KVObject) {
		// Trim the leading & trailing "/" to make it consistent across all stores
		results[strings.Trim(key, "/")] = val
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

// DeleteObjectAtomic performs atomic delete on a record.
func (ds *Store) DeleteObjectAtomic(kvObject KVObject) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if kvObject == nil {
		return types.InvalidParameterErrorf("invalid KV Object : nil")
	}

	previous := &store.KVPair{Key: Key(kvObject.Key()...), LastIndex: kvObject.Index()}

	if !kvObject.Skip() {
		if err := ds.store.AtomicDelete(Key(kvObject.Key()...), previous); err != nil {
			if err == store.ErrKeyExists {
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

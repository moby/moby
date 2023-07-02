package datastore

import (
	"fmt"
	"log"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/libnetwork/discoverapi"
	store "github.com/docker/docker/libnetwork/internal/kvstore"
	"github.com/docker/docker/libnetwork/types"
)

// DataStore exported
type DataStore interface {
	// GetObject gets data from datastore and unmarshals to the specified object
	GetObject(key string, o KVObject) error
	// PutObject adds a new Record based on an object into the datastore
	PutObject(kvObject KVObject) error
	// PutObjectAtomic provides an atomic add and update operation for a Record
	PutObjectAtomic(kvObject KVObject) error
	// DeleteObject deletes a record
	DeleteObject(kvObject KVObject) error
	// DeleteObjectAtomic performs an atomic delete operation
	DeleteObjectAtomic(kvObject KVObject) error
	// DeleteTree deletes a record
	DeleteTree(kvObject KVObject) error
	// Watchable returns whether the store is watchable or not
	Watchable() bool
	// Watch for changes on a KVObject
	Watch(kvObject KVObject, stopCh <-chan struct{}) (<-chan KVObject, error)
	// RestartWatch retriggers stopped Watches
	RestartWatch()
	// Active returns if the store is active
	Active() bool
	// List returns of a list of KVObjects belonging to the parent
	// key. The caller must pass a KVObject of the same type as
	// the objects that need to be listed
	List(string, KVObject) ([]KVObject, error)
	// Map returns a Map of KVObjects
	Map(key string, kvObject KVObject) (map[string]KVObject, error)
	// Scope returns the scope of the store
	Scope() string
	// KVStore returns access to the KV Store
	KVStore() store.Store
	// Close closes the data store
	Close()
}

// ErrKeyModified is raised for an atomic update when the update is working on a stale state
var (
	ErrKeyModified = store.ErrKeyModified
	ErrKeyNotFound = store.ErrKeyNotFound
)

type datastore struct {
	scope      string
	store      store.Store
	cache      *cache
	watchCh    chan struct{}
	active     bool
	sequential bool
	sync.Mutex
}

// KVObject is Key/Value interface used by objects to be part of the DataStore
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
	// True if the object exists in the datastore, false if it hasn't been stored yet.
	// When SetIndex() is called, the object has been stored.
	Exists() bool
	// DataScope indicates the storage scope of the KV object
	DataScope() string
	// Skip provides a way for a KV Object to avoid persisting it in the KV Store
	Skip() bool
}

// KVConstructor interface defines methods which can construct a KVObject from another.
type KVConstructor interface {
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
	// LocalScope indicates to store the KV object in local datastore such as boltdb
	LocalScope = "local"
	// GlobalScope indicates to store the KV object in global datastore
	GlobalScope = "global"
	// SwarmScope is not indicating a datastore location. It is defined here
	// along with the other two scopes just for consistency.
	SwarmScope    = "swarm"
	defaultPrefix = "/var/lib/docker/network/files"
)

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
			},
		},
	}
}

// IsValid checks if the scope config has valid configuration.
func (cfg *ScopeCfg) IsValid() bool {
	if cfg == nil ||
		strings.TrimSpace(cfg.Client.Provider) == "" ||
		strings.TrimSpace(cfg.Client.Address) == "" {
		return false
	}

	return true
}

// Key provides convenient method to create a Key
func Key(key ...string) string {
	keychain := append(rootChain, key...)
	str := strings.Join(keychain, "/")
	return str + "/"
}

// ParseKey provides convenient method to unpack the key to complement the Key function
func ParseKey(key string) ([]string, error) {
	chain := strings.Split(strings.Trim(key, "/"), "/")

	// The key must at least be equal to the rootChain in order to be considered as valid
	if len(chain) <= len(rootChain) || !reflect.DeepEqual(chain[0:len(rootChain)], rootChain) {
		return nil, types.BadRequestErrorf("invalid Key : %s", key)
	}
	return chain[len(rootChain):], nil
}

// newClient used to connect to KV Store
func newClient(kv string, addr string, config *store.Config) (DataStore, error) {
	if config == nil {
		config = &store.Config{}
	}

	var addrs []string

	if kv == string(store.BOLTDB) {
		// Parse file path
		addrs = strings.Split(addr, ",")
	} else {
		// Parse URI
		parts := strings.SplitN(addr, "/", 2)
		addrs = strings.Split(parts[0], ",")

		// Add the custom prefix to the root chain
		if len(parts) == 2 {
			rootChain = append([]string{parts[1]}, defaultRootChain...)
		}
	}

	s, err := store.New(store.Backend(kv), addrs, config)
	if err != nil {
		return nil, err
	}

	ds := &datastore{scope: LocalScope, store: s, active: true, watchCh: make(chan struct{}), sequential: true}
	ds.cache = newCache(ds)

	return ds, nil
}

// NewDataStore creates a new instance of LibKV data store
func NewDataStore(cfg ScopeCfg) (DataStore, error) {
	if cfg.Client.Provider == "" || cfg.Client.Address == "" {
		cfg = DefaultScope("")
	}

	return newClient(cfg.Client.Provider, cfg.Client.Address, cfg.Client.Config)
}

// NewDataStoreFromConfig creates a new instance of LibKV data store starting from the datastore config data
func NewDataStoreFromConfig(dsc discoverapi.DatastoreConfigData) (DataStore, error) {
	var (
		ok    bool
		sCfgP *store.Config
	)

	sCfgP, ok = dsc.Config.(*store.Config)
	if !ok && dsc.Config != nil {
		return nil, fmt.Errorf("cannot parse store configuration: %v", dsc.Config)
	}

	scopeCfg := ScopeCfg{
		Client: ScopeClientCfg{
			Address:  dsc.Address,
			Provider: dsc.Provider,
			Config:   sCfgP,
		},
	}

	ds, err := NewDataStore(scopeCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to construct datastore client from datastore configuration %v: %v", dsc, err)
	}

	return ds, err
}

func (ds *datastore) Close() {
	ds.store.Close()
}

func (ds *datastore) Scope() string {
	return ds.scope
}

func (ds *datastore) Active() bool {
	return ds.active
}

func (ds *datastore) Watchable() bool {
	return ds.scope != LocalScope
}

func (ds *datastore) Watch(kvObject KVObject, stopCh <-chan struct{}) (<-chan KVObject, error) {
	sCh := make(chan struct{})

	ctor, ok := kvObject.(KVConstructor)
	if !ok {
		return nil, fmt.Errorf("error watching object type %T, object does not implement KVConstructor interface", kvObject)
	}

	kvpCh, err := ds.store.Watch(Key(kvObject.Key()...), sCh)
	if err != nil {
		return nil, err
	}

	kvoCh := make(chan KVObject)

	go func() {
	retry_watch:
		var err error

		// Make sure to get a new instance of watch channel
		ds.Lock()
		watchCh := ds.watchCh
		ds.Unlock()

	loop:
		for {
			select {
			case <-stopCh:
				close(sCh)
				return
			case kvPair := <-kvpCh:
				// If the backend KV store gets reset libkv's go routine
				// for the watch can exit resulting in a nil value in
				// channel.
				if kvPair == nil {
					ds.Lock()
					ds.active = false
					ds.Unlock()
					break loop
				}

				dstO := ctor.New()

				if err = dstO.SetValue(kvPair.Value); err != nil {
					log.Printf("Could not unmarshal kvpair value = %s", string(kvPair.Value))
					break
				}

				dstO.SetIndex(kvPair.LastIndex)
				kvoCh <- dstO
			}
		}

		// Wait on watch channel for a re-trigger when datastore becomes active
		<-watchCh

		kvpCh, err = ds.store.Watch(Key(kvObject.Key()...), sCh)
		if err != nil {
			log.Printf("Could not watch the key %s in store: %v", Key(kvObject.Key()...), err)
		}

		goto retry_watch
	}()

	return kvoCh, nil
}

func (ds *datastore) RestartWatch() {
	ds.Lock()
	defer ds.Unlock()

	ds.active = true
	watchCh := ds.watchCh
	ds.watchCh = make(chan struct{})
	close(watchCh)
}

func (ds *datastore) KVStore() store.Store {
	return ds.store
}

// PutObjectAtomic adds a new Record based on an object into the datastore
func (ds *datastore) PutObjectAtomic(kvObject KVObject) error {
	var (
		previous *store.KVPair
		pair     *store.KVPair
		err      error
	)
	if ds.sequential {
		ds.Lock()
		defer ds.Unlock()
	}

	if kvObject == nil {
		return types.BadRequestErrorf("invalid KV Object : nil")
	}

	kvObjValue := kvObject.Value()

	if kvObjValue == nil {
		return types.BadRequestErrorf("invalid KV Object with a nil Value for key %s", Key(kvObject.Key()...))
	}

	if kvObject.Skip() {
		goto add_cache
	}

	if kvObject.Exists() {
		previous = &store.KVPair{Key: Key(kvObject.Key()...), LastIndex: kvObject.Index()}
	} else {
		previous = nil
	}

	pair, err = ds.store.AtomicPut(Key(kvObject.Key()...), kvObjValue, previous)
	if err != nil {
		if err == store.ErrKeyExists {
			return ErrKeyModified
		}
		return err
	}

	kvObject.SetIndex(pair.LastIndex)

add_cache:
	if ds.cache != nil {
		// If persistent store is skipped, sequencing needs to
		// happen in cache.
		return ds.cache.add(kvObject, kvObject.Skip())
	}

	return nil
}

// PutObject adds a new Record based on an object into the datastore
func (ds *datastore) PutObject(kvObject KVObject) error {
	if ds.sequential {
		ds.Lock()
		defer ds.Unlock()
	}

	if kvObject == nil {
		return types.BadRequestErrorf("invalid KV Object : nil")
	}

	if kvObject.Skip() {
		goto add_cache
	}

	if err := ds.putObjectWithKey(kvObject, kvObject.Key()...); err != nil {
		return err
	}

add_cache:
	if ds.cache != nil {
		// If persistent store is skipped, sequencing needs to
		// happen in cache.
		return ds.cache.add(kvObject, kvObject.Skip())
	}

	return nil
}

func (ds *datastore) putObjectWithKey(kvObject KVObject, key ...string) error {
	kvObjValue := kvObject.Value()

	if kvObjValue == nil {
		return types.BadRequestErrorf("invalid KV Object with a nil Value for key %s", Key(kvObject.Key()...))
	}
	return ds.store.Put(Key(key...), kvObjValue)
}

// GetObject returns a record matching the key
func (ds *datastore) GetObject(key string, o KVObject) error {
	if ds.sequential {
		ds.Lock()
		defer ds.Unlock()
	}

	if ds.cache != nil {
		return ds.cache.get(key, o)
	}

	kvPair, err := ds.store.Get(key)
	if err != nil {
		return err
	}

	if err := o.SetValue(kvPair.Value); err != nil {
		return err
	}

	// Make sure the object has a correct view of the DB index in
	// case we need to modify it and update the DB.
	o.SetIndex(kvPair.LastIndex)
	return nil
}

func (ds *datastore) ensureParent(parent string) error {
	exists, err := ds.store.Exists(parent)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return ds.store.Put(parent, []byte{})
}

func (ds *datastore) List(key string, kvObject KVObject) ([]KVObject, error) {
	if ds.sequential {
		ds.Lock()
		defer ds.Unlock()
	}

	if ds.cache != nil {
		return ds.cache.list(kvObject)
	}

	var kvol []KVObject
	cb := func(key string, val KVObject) {
		kvol = append(kvol, val)
	}
	err := ds.iterateKVPairsFromStore(key, kvObject, cb)
	if err != nil {
		return nil, err
	}
	return kvol, nil
}

func (ds *datastore) iterateKVPairsFromStore(key string, kvObject KVObject, callback func(string, KVObject)) error {
	// Bail out right away if the kvObject does not implement KVConstructor
	ctor, ok := kvObject.(KVConstructor)
	if !ok {
		return fmt.Errorf("error listing objects, object does not implement KVConstructor interface")
	}

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

func (ds *datastore) Map(key string, kvObject KVObject) (map[string]KVObject, error) {
	if ds.sequential {
		ds.Lock()
		defer ds.Unlock()
	}

	kvol := make(map[string]KVObject)
	cb := func(key string, val KVObject) {
		// Trim the leading & trailing "/" to make it consistent across all stores
		kvol[strings.Trim(key, "/")] = val
	}
	err := ds.iterateKVPairsFromStore(key, kvObject, cb)
	if err != nil {
		return nil, err
	}
	return kvol, nil
}

// DeleteObject unconditionally deletes a record from the store
func (ds *datastore) DeleteObject(kvObject KVObject) error {
	if ds.sequential {
		ds.Lock()
		defer ds.Unlock()
	}

	// cleanup the cache first
	if ds.cache != nil {
		// If persistent store is skipped, sequencing needs to
		// happen in cache.
		ds.cache.del(kvObject, kvObject.Skip())
	}

	if kvObject.Skip() {
		return nil
	}

	return ds.store.Delete(Key(kvObject.Key()...))
}

// DeleteObjectAtomic performs atomic delete on a record
func (ds *datastore) DeleteObjectAtomic(kvObject KVObject) error {
	if ds.sequential {
		ds.Lock()
		defer ds.Unlock()
	}

	if kvObject == nil {
		return types.BadRequestErrorf("invalid KV Object : nil")
	}

	previous := &store.KVPair{Key: Key(kvObject.Key()...), LastIndex: kvObject.Index()}

	if kvObject.Skip() {
		goto del_cache
	}

	if err := ds.store.AtomicDelete(Key(kvObject.Key()...), previous); err != nil {
		if err == store.ErrKeyExists {
			return ErrKeyModified
		}
		return err
	}

del_cache:
	// cleanup the cache only if AtomicDelete went through successfully
	if ds.cache != nil {
		// If persistent store is skipped, sequencing needs to
		// happen in cache.
		return ds.cache.del(kvObject, kvObject.Skip())
	}

	return nil
}

// DeleteTree unconditionally deletes a record from the store
func (ds *datastore) DeleteTree(kvObject KVObject) error {
	if ds.sequential {
		ds.Lock()
		defer ds.Unlock()
	}

	// cleanup the cache first
	if ds.cache != nil {
		// If persistent store is skipped, sequencing needs to
		// happen in cache.
		ds.cache.del(kvObject, kvObject.Skip())
	}

	if kvObject.Skip() {
		return nil
	}

	return ds.store.DeleteTree(Key(kvObject.KeyPrefix()...))
}

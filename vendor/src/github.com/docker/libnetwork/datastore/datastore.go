package datastore

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/docker/libkv"
	"github.com/docker/libkv/store"
	"github.com/docker/libnetwork/config"
	"github.com/docker/libnetwork/types"
)

//DataStore exported
type DataStore interface {
	// GetObject gets data from datastore and unmarshals to the specified object
	GetObject(key string, o interface{}) error
	// PutObject adds a new Record based on an object into the datastore
	PutObject(kvObject KV) error
	// PutObjectAtomic provides an atomic add and update operation for a Record
	PutObjectAtomic(kvObject KV) error
	// DeleteObject deletes a record
	DeleteObject(kvObject KV) error
	// DeleteObjectAtomic performs an atomic delete operation
	DeleteObjectAtomic(kvObject KV) error
	// DeleteTree deletes a record
	DeleteTree(kvObject KV) error
	// KVStore returns access to the KV Store
	KVStore() store.Store
}

// ErrKeyModified is raised for an atomic update when the update is working on a stale state
var ErrKeyModified = store.ErrKeyModified

type datastore struct {
	store store.Store
}

//KV Key Value interface used by objects to be part of the DataStore
type KV interface {
	// Key method lets an object to provide the Key to be used in KV Store
	Key() []string
	// KeyPrefix method lets an object to return immediate parent key that can be used for tree walk
	KeyPrefix() []string
	// Value method lets an object to marshal its content to be stored in the KV store
	Value() []byte
	// Index method returns the latest DB Index as seen by the object
	Index() uint64
	// SetIndex method allows the datastore to store the latest DB Index into the object
	SetIndex(uint64)
}

const (
	// NetworkKeyPrefix is the prefix for network key in the kv store
	NetworkKeyPrefix = "network"
	// EndpointKeyPrefix is the prefix for endpoint key in the kv store
	EndpointKeyPrefix = "endpoint"
)

var rootChain = []string{"docker", "libnetwork"}

//Key provides convenient method to create a Key
func Key(key ...string) string {
	keychain := append(rootChain, key...)
	str := strings.Join(keychain, "/")
	return str + "/"
}

//ParseKey provides convenient method to unpack the key to complement the Key function
func ParseKey(key string) ([]string, error) {
	chain := strings.Split(strings.Trim(key, "/"), "/")

	// The key must atleast be equal to the rootChain in order to be considered as valid
	if len(chain) <= len(rootChain) || !reflect.DeepEqual(chain[0:len(rootChain)], rootChain) {
		return nil, types.BadRequestErrorf("invalid Key : %s", key)
	}
	return chain[len(rootChain):], nil
}

// newClient used to connect to KV Store
func newClient(kv string, addrs string) (DataStore, error) {
	store, err := libkv.NewStore(store.Backend(kv), []string{addrs}, &store.Config{})
	if err != nil {
		return nil, err
	}
	ds := &datastore{store: store}
	return ds, nil
}

// NewDataStore creates a new instance of LibKV data store
func NewDataStore(cfg *config.DatastoreCfg) (DataStore, error) {
	if cfg == nil {
		return nil, types.BadRequestErrorf("invalid configuration passed to datastore")
	}
	// TODO : cfg.Embedded case
	return newClient(cfg.Client.Provider, cfg.Client.Address)
}

// NewCustomDataStore can be used by clients to plugin cusom datatore that adhers to store.Store
func NewCustomDataStore(customStore store.Store) DataStore {
	return &datastore{store: customStore}
}

func (ds *datastore) KVStore() store.Store {
	return ds.store
}

// PutObjectAtomic adds a new Record based on an object into the datastore
func (ds *datastore) PutObjectAtomic(kvObject KV) error {
	if kvObject == nil {
		return types.BadRequestErrorf("invalid KV Object : nil")
	}
	kvObjValue := kvObject.Value()

	if kvObjValue == nil {
		return types.BadRequestErrorf("invalid KV Object with a nil Value for key %s", Key(kvObject.Key()...))
	}

	previous := &store.KVPair{Key: Key(kvObject.Key()...), LastIndex: kvObject.Index()}
	_, pair, err := ds.store.AtomicPut(Key(kvObject.Key()...), kvObjValue, previous, nil)
	if err != nil {
		return err
	}

	kvObject.SetIndex(pair.LastIndex)
	return nil
}

// PutObject adds a new Record based on an object into the datastore
func (ds *datastore) PutObject(kvObject KV) error {
	if kvObject == nil {
		return types.BadRequestErrorf("invalid KV Object : nil")
	}
	return ds.putObjectWithKey(kvObject, kvObject.Key()...)
}

func (ds *datastore) putObjectWithKey(kvObject KV, key ...string) error {
	kvObjValue := kvObject.Value()

	if kvObjValue == nil {
		return types.BadRequestErrorf("invalid KV Object with a nil Value for key %s", Key(kvObject.Key()...))
	}
	return ds.store.Put(Key(key...), kvObjValue, nil)
}

// GetObject returns a record matching the key
func (ds *datastore) GetObject(key string, o interface{}) error {
	kvPair, err := ds.store.Get(key)
	if err != nil {
		return err
	}
	return json.Unmarshal(kvPair.Value, o)
}

// DeleteObject unconditionally deletes a record from the store
func (ds *datastore) DeleteObject(kvObject KV) error {
	return ds.store.Delete(Key(kvObject.Key()...))
}

// DeleteObjectAtomic performs atomic delete on a record
func (ds *datastore) DeleteObjectAtomic(kvObject KV) error {
	if kvObject == nil {
		return types.BadRequestErrorf("invalid KV Object : nil")
	}

	previous := &store.KVPair{Key: Key(kvObject.Key()...), LastIndex: kvObject.Index()}
	_, err := ds.store.AtomicDelete(Key(kvObject.Key()...), previous)
	return err
}

// DeleteTree unconditionally deletes a record from the store
func (ds *datastore) DeleteTree(kvObject KV) error {
	return ds.store.DeleteTree(Key(kvObject.KeyPrefix()...))
}

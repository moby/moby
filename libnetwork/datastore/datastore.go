package datastore

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/docker/libnetwork/config"
	"github.com/docker/swarm/pkg/store"
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

//Key provides convenient method to create a Key
func Key(key ...string) string {
	keychain := []string{"docker", "libnetwork"}
	keychain = append(keychain, key...)
	str := strings.Join(keychain, "/")
	return str + "/"
}

var errNewDatastore = errors.New("Error creating new Datastore")
var errInvalidConfiguration = errors.New("Invalid Configuration passed to Datastore")
var errInvalidAtomicRequest = errors.New("Invalid Atomic Request")

// newClient used to connect to KV Store
func newClient(kv string, addrs string) (DataStore, error) {
	store, err := store.NewStore(store.Backend(kv), []string{addrs}, &store.Config{})
	if err != nil {
		return nil, err
	}
	ds := &datastore{store: store}
	return ds, nil
}

// NewDataStore creates a new instance of LibKV data store
func NewDataStore(cfg *config.DatastoreCfg) (DataStore, error) {
	if cfg == nil {
		return nil, errInvalidConfiguration
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
		return errors.New("kvObject is nil")
	}
	kvObjValue := kvObject.Value()

	if kvObjValue == nil {
		return errInvalidAtomicRequest
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
		return errors.New("kvObject is nil")
	}
	return ds.putObjectWithKey(kvObject, kvObject.Key()...)
}

func (ds *datastore) putObjectWithKey(kvObject KV, key ...string) error {
	kvObjValue := kvObject.Value()

	if kvObjValue == nil {
		return errors.New("Object must provide marshalled data for key : " + Key(kvObject.Key()...))
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
		return errors.New("kvObject is nil")
	}

	previous := &store.KVPair{Key: Key(kvObject.Key()...), LastIndex: kvObject.Index()}
	_, err := ds.store.AtomicDelete(Key(kvObject.Key()...), previous)
	if err != nil {
		return err
	}

	return nil
}

// DeleteTree unconditionally deletes a record from the store
func (ds *datastore) DeleteTree(kvObject KV) error {
	return ds.store.DeleteTree(Key(kvObject.KeyPrefix()...))
}

package datastore

import (
	"errors"
	"strings"

	"github.com/docker/swarm/pkg/store"
)

//DataStore exported
type DataStore interface {
	// PutObject adds a new Record based on an object into the datastore
	PutObject(kvObject KV) error
	// PutObjectAtomic provides an atomic add and update operation for a Record
	PutObjectAtomic(kvObject KV) error
	// KVStore returns access to the KV Store
	KVStore() store.Store
}

type datastore struct {
	store  store.Store
	config *StoreConfiguration
}

//StoreConfiguration exported
type StoreConfiguration struct {
	Addrs    []string
	Provider string
}

//KV Key Value interface used by objects to be part of the DataStore
type KV interface {
	Key() []string
	Value() []byte
	Index() uint64
	SetIndex(uint64)
}

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
func newClient(kv string, addrs []string) (DataStore, error) {
	store, err := store.CreateStore(kv, addrs, store.Config{})
	if err != nil {
		return nil, err
	}
	ds := &datastore{store: store}
	return ds, nil
}

// NewDataStore creates a new instance of LibKV data store
func NewDataStore(config *StoreConfiguration) (DataStore, error) {
	if config == nil {
		return nil, errInvalidConfiguration
	}
	return newClient(config.Provider, config.Addrs)
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
	_, err := ds.store.AtomicPut(Key(kvObject.Key()...), []byte{}, kvObjValue, kvObject.Index())
	if err != nil {
		return err
	}

	_, index, err := ds.store.Get(Key(kvObject.Key()...))
	if err != nil {
		return err
	}
	kvObject.SetIndex(index)
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
	return ds.store.Put(Key(key...), kvObjValue)
}

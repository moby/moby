package kvstore

import (
	"errors"
	"time"
)

// Backend represents a KV Store Backend
type Backend string

// BOLTDB backend
const BOLTDB Backend = "boltdb"

var (
	// ErrBackendNotSupported is thrown when the backend k/v store is not supported by libkv
	ErrBackendNotSupported = errors.New("Backend storage not supported yet, please choose one of")
	// ErrCallNotSupported is thrown when a method is not implemented/supported by the current backend
	ErrCallNotSupported = errors.New("The current call is not supported with this backend")
	// ErrKeyModified is thrown during an atomic operation if the index does not match the one in the store
	ErrKeyModified = errors.New("Unable to complete atomic operation, key modified")
	// ErrKeyNotFound is thrown when the key is not found in the store during a Get operation
	ErrKeyNotFound = errors.New("Key not found in store")
	// ErrPreviousNotSpecified is thrown when the previous value is not specified for an atomic operation
	ErrPreviousNotSpecified = errors.New("Previous K/V pair should be provided for the Atomic operation")
	// ErrKeyExists is thrown when the previous value exists in the case of an AtomicPut
	ErrKeyExists = errors.New("Previous K/V pair exists, cannot complete Atomic operation")
)

// Config contains the options for a storage client
type Config struct {
	ConnectionTimeout time.Duration
	Bucket            string
	PersistConnection bool
}

// Store represents the backend K/V storage
// Each store should support every call listed
// here. Or it couldn't be implemented as a K/V
// backend for libkv
type Store interface {
	// Put a value at the specified key
	Put(key string, value []byte) error

	// Get a value given its key
	Get(key string) (*KVPair, error)

	// Delete the value at the specified key
	Delete(key string) error

	// Exists verifies if a Key exists in the store.
	Exists(key string) (bool, error)

	// Watch for changes on a key
	Watch(key string, stopCh <-chan struct{}) (<-chan *KVPair, error)

	// List the content of a given prefix
	List(directory string) ([]*KVPair, error)

	// AtomicPut performs an atomic CAS operation on a single value.
	// Pass previous = nil to create a new key.
	AtomicPut(key string, value []byte, previous *KVPair) (*KVPair, error)

	// AtomicDelete performs an atomic delete of a single value.
	AtomicDelete(key string, previous *KVPair) error

	// Close the store connection
	Close()
}

// KVPair represents {Key, Value, Lastindex} tuple
type KVPair struct {
	Key       string
	Value     []byte
	LastIndex uint64
}

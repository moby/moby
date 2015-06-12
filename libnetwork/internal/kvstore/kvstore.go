package store

import (
	"crypto/tls"
	"errors"
	"time"
)

// Backend represents a KV Store Backend
type Backend string

const (
	// CONSUL backend
	CONSUL = "consul"
	// ETCD backend
	ETCD = "etcd"
	// ZK backend
	ZK = "zk"
)

var (
	// ErrNotSupported is thrown when the backend k/v store is not supported by libkv
	ErrNotSupported = errors.New("Backend storage not supported yet, please choose another one")
	// ErrNotImplemented is thrown when a method is not implemented by the current backend
	ErrNotImplemented = errors.New("Call not implemented in current backend")
	// ErrNotReachable is thrown when the API cannot be reached for issuing common store operations
	ErrNotReachable = errors.New("Api not reachable")
	// ErrCannotLock is thrown when there is an error acquiring a lock on a key
	ErrCannotLock = errors.New("Error acquiring the lock")
	// ErrKeyModified is thrown during an atomic operation if the index does not match the one in the store
	ErrKeyModified = errors.New("Unable to complete atomic operation, key modified")
	// ErrKeyNotFound is thrown when the key is not found in the store during a Get operation
	ErrKeyNotFound = errors.New("Key not found in store")
	// ErrPreviousNotSpecified is thrown when the previous value is not specified for an atomic operation
	ErrPreviousNotSpecified = errors.New("Previous K/V pair should be provided for the Atomic operation")
)

// Config contains the options for a storage client
type Config struct {
	TLS               *tls.Config
	ConnectionTimeout time.Duration
	EphemeralTTL      time.Duration
}

// Store represents the backend K/V storage
// Each store should support every call listed
// here. Or it couldn't be implemented as a K/V
// backend for libkv
type Store interface {
	// Put a value at the specified key
	Put(key string, value []byte, options *WriteOptions) error

	// Get a value given its key
	Get(key string) (*KVPair, error)

	// Delete the value at the specified key
	Delete(key string) error

	// Verify if a Key exists in the store
	Exists(key string) (bool, error)

	// Watch for changes on a key
	Watch(key string, stopCh <-chan struct{}) (<-chan *KVPair, error)

	// WatchTree watches for changes on child nodes under
	// a given a directory
	WatchTree(directory string, stopCh <-chan struct{}) (<-chan []*KVPair, error)

	// CreateLock for a given key.
	// The returned Locker is not held and must be acquired
	// with `.Lock`. The Value is optional.
	NewLock(key string, options *LockOptions) (Locker, error)

	// List the content of a given prefix
	List(directory string) ([]*KVPair, error)

	// DeleteTree deletes a range of keys under a given directory
	DeleteTree(directory string) error

	// Atomic operation on a single value
	AtomicPut(key string, value []byte, previous *KVPair, options *WriteOptions) (bool, *KVPair, error)

	// Atomic delete of a single value
	AtomicDelete(key string, previous *KVPair) (bool, error)

	// Close the store connection
	Close()
}

// KVPair represents {Key, Value, Lastindex} tuple
type KVPair struct {
	Key       string
	Value     []byte
	LastIndex uint64
}

// WriteOptions contains optional request parameters
type WriteOptions struct {
	Heartbeat time.Duration
	Ephemeral bool
}

// LockOptions contains optional request parameters
type LockOptions struct {
	Value []byte        // Optional, value to associate with the lock
	TTL   time.Duration // Optional, expiration ttl associated with the lock
}

// WatchCallback is used for watch methods on keys
// and is triggered on key change
type WatchCallback func(entries ...*KVPair)

// Locker provides locking mechanism on top of the store.
// Similar to `sync.Lock` except it may return errors.
type Locker interface {
	Lock() (<-chan struct{}, error)
	Unlock() error
}

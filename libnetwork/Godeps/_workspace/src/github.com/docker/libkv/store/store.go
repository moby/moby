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
	CONSUL Backend = "consul"
	// ETCD backend
	ETCD Backend = "etcd"
	// ZK backend
	ZK Backend = "zk"
	// BOLTDB backend
	BOLTDB Backend = "boltdb"
)

var (
	// ErrNotSupported is thrown when the backend k/v store is not supported by libkv
	ErrNotSupported = errors.New("Backend storage not supported yet, please choose one of")
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
	ClientTLS         *ClientTLSConfig
	TLS               *tls.Config
	ConnectionTimeout time.Duration
	Bucket            string
}

// ClientTLSConfig contains data for a Client TLS configuration in the form
//  the etcd client wants it.  Eventually we'll adapt it for ZK and Consul.
type ClientTLSConfig struct {
	CertFile   string
	KeyFile    string
	CACertFile string
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
	// a given directory
	WatchTree(directory string, stopCh <-chan struct{}) (<-chan []*KVPair, error)

	// NewLock creates a lock for a given key.
	// The returned Locker is not held and must be acquired
	// with `.Lock`. The Value is optional.
	NewLock(key string, options *LockOptions) (Locker, error)

	// List the content of a given prefix
	List(directory string) ([]*KVPair, error)

	// DeleteTree deletes a range of keys under a given directory
	DeleteTree(directory string) error

	// Atomic CAS operation on a single value.
	// Pass previous = nil to create a new key.
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
	TTL time.Duration
}

// LockOptions contains optional request parameters
type LockOptions struct {
	Value []byte        // Optional, value to associate with the lock
	TTL   time.Duration // Optional, expiration ttl associated with the lock
}

// Locker provides locking mechanism on top of the store.
// Similar to `sync.Lock` except it may return errors.
type Locker interface {
	Lock() (<-chan struct{}, error)
	Unlock() error
}

package store

import (
	"time"

	log "github.com/Sirupsen/logrus"
)

// WatchCallback is used for watch methods on keys
// and is triggered on key change
type WatchCallback func(kviTuple []KVEntry)

// Initialize creates a new Store object, initializing the client
type Initialize func(addrs []string, options Config) (Store, error)

// Store represents the backend K/V storage
// Each store should support every call listed
// here. Or it couldn't be implemented as a K/V
// backend for libkv
type Store interface {
	// Put a value at the specified key
	Put(key string, value []byte) error

	// Get a value given its key
	Get(key string) (value []byte, lastIndex uint64, err error)

	// Delete the value at the specified key
	Delete(key string) error

	// Verify if a Key exists in the store
	Exists(key string) (bool, error)

	// Watch changes on a key
	Watch(key string, heartbeat time.Duration, callback WatchCallback) error

	// Cancel watch key
	CancelWatch(key string) error

	// Acquire the lock at key
	Acquire(key string, value []byte) (string, error)

	// Release the lock at key
	Release(session string) error

	// Get range of keys based on prefix
	GetRange(prefix string) ([]KVEntry, error)

	// Delete range of keys based on prefix
	DeleteRange(prefix string) error

	// Watch key namespaces
	WatchRange(prefix string, filter string, heartbeat time.Duration, callback WatchCallback) error

	// Cancel watch key range
	CancelWatchRange(prefix string) error

	// Atomic operation on a single value
	AtomicPut(key string, oldValue []byte, newValue []byte, index uint64) (bool, error)

	// Atomic delete of a single value
	AtomicDelete(key string, oldValue []byte, index uint64) (bool, error)
}

// KVEntry represents {Key, Value, Lastindex} tuple
type KVEntry interface {
	Key() string
	Value() []byte
	LastIndex() uint64
}

var (
	// List of Store services
	stores map[string]Initialize
)

func init() {
	stores = make(map[string]Initialize)
	stores["consul"] = InitializeConsul
	stores["etcd"] = InitializeEtcd
	stores["zk"] = InitializeZookeeper
}

// CreateStore creates a an instance of store
func CreateStore(store string, addrs []string, options Config) (Store, error) {

	if init, exists := stores[store]; exists {
		log.WithFields(log.Fields{"store": store}).Debug("Initializing store service")
		return init(addrs, options)
	}

	return nil, ErrNotSupported
}

# Storage

This package is used by the discovery service to register machines inside the cluster. It is also used to store cluster's metadata.

## Example of usage

### Create a new store and use Put/Get

```go
package main

import (
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/swarm/store"
)

func main() {
	var (
		client = "localhost:8500"
	)

	// Initialize a new store with consul
	kv, err := store.CreateStore(
		store.Consul,
		[]string{client},
		store.Config{
		    Timeout: 10*time.Second
		},
	)
	if err != nil {
		log.Error("Cannot create store consul")
	}

	key := "foo"
	err = kv.Put(key, []byte("bar"))
	if err != nil {
		log.Error("Error trying to put value at key `", key, "`")
	}

	value, _, err := kv.Get(key)
	if err != nil {
		log.Error("Error trying accessing value at key `", key, "`")
	}

	log.Info("value: ", string(value))
}
```



## Contributing to a new storage backend

A new **storage backend** should include those calls:

```go
type Store interface {
	Put(key string, value []byte) error
	Get(key string) (value []byte, lastIndex uint64, err error)
	Delete(key string) error
	Exists(key string) (bool, error)
	Watch(key string, ttl uint64, callback WatchCallback) error
	CancelWatch(key string) error
	Acquire(key string, value []byte) (string, error)
	Release(session string) error
	GetRange(prefix string) (value [][]byte, err error)
	DeleteRange(prefix string) error
	WatchRange(prefix string, filter string, heartbeat uint64, callback WatchCallback) error
	CancelWatchRange(prefix string) error
	AtomicPut(key string, oldValue []byte, newValue []byte, index uint64) (bool, error)
	AtomicDelete(key string, oldValue []byte, index uint64) (bool, error)
}
```

To be elligible as a **discovery backend** only, a K/V store implementation should at least offer `Get`, `Put`, `WatchRange`, `GetRange`.

You can get inspiration from existing backends to create a new one. This interface could be subject to changes to improve the experience of using the library and contributing to a new backend.

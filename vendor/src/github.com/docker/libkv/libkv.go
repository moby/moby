package libkv

import (
	"github.com/docker/libkv/store"
	"github.com/docker/libkv/store/consul"
	"github.com/docker/libkv/store/etcd"
	"github.com/docker/libkv/store/zookeeper"
)

// Initialize creates a new Store object, initializing the client
type Initialize func(addrs []string, options *store.Config) (store.Store, error)

var (
	// Backend initializers
	initializers = map[store.Backend]Initialize{
		store.CONSUL: consul.New,
		store.ETCD:   etcd.New,
		store.ZK:     zookeeper.New,
	}
)

// NewStore creates a an instance of store
func NewStore(backend store.Backend, addrs []string, options *store.Config) (store.Store, error) {
	if init, exists := initializers[backend]; exists {
		return init(addrs, options)
	}

	return nil, store.ErrNotSupported
}

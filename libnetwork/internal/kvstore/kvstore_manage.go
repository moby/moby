package kvstore

import (
	"fmt"
	"sort"
	"strings"
)

// Initialize creates a new Store object, initializing the client
type Initialize func(addrs []string, options *Config) (Store, error)

var (
	// Backend initializers
	initializers = make(map[Backend]Initialize)

	supportedBackend = func() string {
		keys := make([]string, 0, len(initializers))
		for k := range initializers {
			keys = append(keys, string(k))
		}
		sort.Strings(keys)
		return strings.Join(keys, ", ")
	}()
)

// New creates an instance of store
func New(backend Backend, addrs []string, options *Config) (Store, error) {
	if init, exists := initializers[backend]; exists {
		return init(addrs, options)
	}

	return nil, fmt.Errorf("%s %s", ErrBackendNotSupported.Error(), supportedBackend)
}

// AddStore adds a new store backend to libkv
func AddStore(store Backend, init Initialize) {
	initializers[store] = init
}

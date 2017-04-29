package test

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli/config/credentials"
)

// fake store implements a credentials.Store that only acts as an in memory map
type fakeStore struct {
	store      map[string]types.AuthConfig
	eraseFunc  func(serverAddress string) error
	getFunc    func(serverAddress string) (types.AuthConfig, error)
	getAllFunc func() (map[string]types.AuthConfig, error)
	storeFunc  func(authConfig types.AuthConfig) error
}

// NewFakeStore creates a new file credentials store.
func NewFakeStore() credentials.Store {
	return &fakeStore{store: map[string]types.AuthConfig{}}
}

func (c *fakeStore) SetStore(store map[string]types.AuthConfig) {
	c.store = store
}

func (c *fakeStore) SetEraseFunc(eraseFunc func(string) error) {
	c.eraseFunc = eraseFunc
}

func (c *fakeStore) SetGetFunc(getFunc func(string) (types.AuthConfig, error)) {
	c.getFunc = getFunc
}

func (c *fakeStore) SetGetAllFunc(getAllFunc func() (map[string]types.AuthConfig, error)) {
	c.getAllFunc = getAllFunc
}

func (c *fakeStore) SetStoreFunc(storeFunc func(types.AuthConfig) error) {
	c.storeFunc = storeFunc
}

// Erase removes the given credentials from the map store
func (c *fakeStore) Erase(serverAddress string) error {
	if c.eraseFunc != nil {
		return c.eraseFunc(serverAddress)
	}
	delete(c.store, serverAddress)
	return nil
}

// Get retrieves credentials for a specific server from the map store.
func (c *fakeStore) Get(serverAddress string) (types.AuthConfig, error) {
	if c.getFunc != nil {
		return c.getFunc(serverAddress)
	}
	authConfig, _ := c.store[serverAddress]
	return authConfig, nil
}

func (c *fakeStore) GetAll() (map[string]types.AuthConfig, error) {
	if c.getAllFunc != nil {
		return c.getAllFunc()
	}
	return c.store, nil
}

// Store saves the given credentials in the map store.
func (c *fakeStore) Store(authConfig types.AuthConfig) error {
	if c.storeFunc != nil {
		return c.storeFunc(authConfig)
	}
	c.store[authConfig.ServerAddress] = authConfig
	return nil
}

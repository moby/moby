package mock

import (
	"github.com/docker/libkv/store"
	"github.com/stretchr/testify/mock"
)

// Mock store. Mocks all Store functions using testify.Mock
type Mock struct {
	mock.Mock

	// Endpoints passed to InitializeMock
	Endpoints []string

	// Options passed to InitializeMock
	Options *store.Config
}

// New creates a Mock store
func New(endpoints []string, options *store.Config) (store.Store, error) {
	s := &Mock{}
	s.Endpoints = endpoints
	s.Options = options
	return s, nil
}

// Put mock
func (s *Mock) Put(key string, value []byte, opts *store.WriteOptions) error {
	args := s.Mock.Called(key, value, opts)
	return args.Error(0)
}

// Get mock
func (s *Mock) Get(key string) (*store.KVPair, error) {
	args := s.Mock.Called(key)
	return args.Get(0).(*store.KVPair), args.Error(1)
}

// Delete mock
func (s *Mock) Delete(key string) error {
	args := s.Mock.Called(key)
	return args.Error(0)
}

// Exists mock
func (s *Mock) Exists(key string) (bool, error) {
	args := s.Mock.Called(key)
	return args.Bool(0), args.Error(1)
}

// Watch mock
func (s *Mock) Watch(key string, stopCh <-chan struct{}) (<-chan *store.KVPair, error) {
	args := s.Mock.Called(key, stopCh)
	return args.Get(0).(<-chan *store.KVPair), args.Error(1)
}

// WatchTree mock
func (s *Mock) WatchTree(prefix string, stopCh <-chan struct{}) (<-chan []*store.KVPair, error) {
	args := s.Mock.Called(prefix, stopCh)
	return args.Get(0).(chan []*store.KVPair), args.Error(1)
}

// NewLock mock
func (s *Mock) NewLock(key string, options *store.LockOptions) (store.Locker, error) {
	args := s.Mock.Called(key, options)
	return args.Get(0).(store.Locker), args.Error(1)
}

// List mock
func (s *Mock) List(prefix string) ([]*store.KVPair, error) {
	args := s.Mock.Called(prefix)
	return args.Get(0).([]*store.KVPair), args.Error(1)
}

// DeleteTree mock
func (s *Mock) DeleteTree(prefix string) error {
	args := s.Mock.Called(prefix)
	return args.Error(0)
}

// AtomicPut mock
func (s *Mock) AtomicPut(key string, value []byte, previous *store.KVPair, opts *store.WriteOptions) (bool, *store.KVPair, error) {
	args := s.Mock.Called(key, value, previous, opts)
	return args.Bool(0), args.Get(1).(*store.KVPair), args.Error(2)
}

// AtomicDelete mock
func (s *Mock) AtomicDelete(key string, previous *store.KVPair) (bool, error) {
	args := s.Mock.Called(key, previous)
	return args.Bool(0), args.Error(1)
}

// Lock mock implementation of Locker
type Lock struct {
	mock.Mock
}

// Lock mock
func (l *Lock) Lock(stopCh chan struct{}) (<-chan struct{}, error) {
	args := l.Mock.Called(stopCh)
	return args.Get(0).(<-chan struct{}), args.Error(1)
}

// Unlock mock
func (l *Lock) Unlock() error {
	args := l.Mock.Called()
	return args.Error(0)
}

// Close mock
func (s *Mock) Close() {
	return
}

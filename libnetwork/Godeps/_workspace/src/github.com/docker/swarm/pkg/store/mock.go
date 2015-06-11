package store

import "github.com/stretchr/testify/mock"

// Mock store. Mocks all Store functions using testify.Mock.
type Mock struct {
	mock.Mock

	// Endpoints passed to InitializeMock
	Endpoints []string
	// Options passed to InitializeMock
	Options *Config
}

// InitializeMock creates a Mock store.
func InitializeMock(endpoints []string, options *Config) (Store, error) {
	s := &Mock{}
	s.Endpoints = endpoints
	s.Options = options
	return s, nil
}

// Put mock
func (s *Mock) Put(key string, value []byte, opts *WriteOptions) error {
	args := s.Mock.Called(key, value, opts)
	return args.Error(0)
}

// Get mock
func (s *Mock) Get(key string) (*KVPair, error) {
	args := s.Mock.Called(key)
	return args.Get(0).(*KVPair), args.Error(1)
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
func (s *Mock) Watch(key string, stopCh <-chan struct{}) (<-chan *KVPair, error) {
	args := s.Mock.Called(key, stopCh)
	return args.Get(0).(<-chan *KVPair), args.Error(1)
}

// WatchTree mock
func (s *Mock) WatchTree(prefix string, stopCh <-chan struct{}) (<-chan []*KVPair, error) {
	args := s.Mock.Called(prefix, stopCh)
	return args.Get(0).(chan []*KVPair), args.Error(1)
}

// NewLock mock
func (s *Mock) NewLock(key string, options *LockOptions) (Locker, error) {
	args := s.Mock.Called(key, options)
	return args.Get(0).(Locker), args.Error(1)
}

// List mock
func (s *Mock) List(prefix string) ([]*KVPair, error) {
	args := s.Mock.Called(prefix)
	return args.Get(0).([]*KVPair), args.Error(1)
}

// DeleteTree mock
func (s *Mock) DeleteTree(prefix string) error {
	args := s.Mock.Called(prefix)
	return args.Error(0)
}

// AtomicPut mock
func (s *Mock) AtomicPut(key string, value []byte, previous *KVPair, opts *WriteOptions) (bool, *KVPair, error) {
	args := s.Mock.Called(key, value, previous, opts)
	return args.Bool(0), args.Get(1).(*KVPair), args.Error(2)
}

// AtomicDelete mock
func (s *Mock) AtomicDelete(key string, previous *KVPair) (bool, error) {
	args := s.Mock.Called(key, previous)
	return args.Bool(0), args.Error(1)
}

// MockLock mock implementation of Locker
type MockLock struct {
	mock.Mock
}

// Lock mock
func (l *MockLock) Lock() (<-chan struct{}, error) {
	args := l.Mock.Called()
	return args.Get(0).(<-chan struct{}), args.Error(1)
}

// Unlock mock
func (l *MockLock) Unlock() error {
	args := l.Mock.Called()
	return args.Error(0)
}

// Close mock
func (s *Mock) Close() {
	return
}

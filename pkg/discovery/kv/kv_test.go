package kv

import (
	"errors"
	"path"
	"testing"
	"time"

	"github.com/docker/docker/pkg/discovery"
	"github.com/docker/libkv/store"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DiscoverySuite struct{}

var _ = check.Suite(&DiscoverySuite{})

func (ds *DiscoverySuite) TestInitialize(c *check.C) {
	storeMock := &FakeStore{
		Endpoints: []string{"127.0.0.1"},
	}
	d := &Discovery{backend: store.CONSUL}
	d.Initialize("127.0.0.1", 0, 0)
	d.store = storeMock

	s := d.store.(*FakeStore)
	c.Assert(s.Endpoints, check.HasLen, 1)
	c.Assert(s.Endpoints[0], check.Equals, "127.0.0.1")
	c.Assert(d.path, check.Equals, discoveryPath)

	storeMock = &FakeStore{
		Endpoints: []string{"127.0.0.1:1234"},
	}
	d = &Discovery{backend: store.CONSUL}
	d.Initialize("127.0.0.1:1234/path", 0, 0)
	d.store = storeMock

	s = d.store.(*FakeStore)
	c.Assert(s.Endpoints, check.HasLen, 1)
	c.Assert(s.Endpoints[0], check.Equals, "127.0.0.1:1234")
	c.Assert(d.path, check.Equals, "path/"+discoveryPath)

	storeMock = &FakeStore{
		Endpoints: []string{"127.0.0.1:1234", "127.0.0.2:1234", "127.0.0.3:1234"},
	}
	d = &Discovery{backend: store.CONSUL}
	d.Initialize("127.0.0.1:1234,127.0.0.2:1234,127.0.0.3:1234/path", 0, 0)
	d.store = storeMock

	s = d.store.(*FakeStore)
	c.Assert(s.Endpoints, check.HasLen, 3)
	c.Assert(s.Endpoints[0], check.Equals, "127.0.0.1:1234")
	c.Assert(s.Endpoints[1], check.Equals, "127.0.0.2:1234")
	c.Assert(s.Endpoints[2], check.Equals, "127.0.0.3:1234")

	c.Assert(d.path, check.Equals, "path/"+discoveryPath)
}

func (ds *DiscoverySuite) TestWatch(c *check.C) {
	mockCh := make(chan []*store.KVPair)

	storeMock := &FakeStore{
		Endpoints:  []string{"127.0.0.1:1234"},
		mockKVChan: mockCh,
	}

	d := &Discovery{backend: store.CONSUL}
	d.Initialize("127.0.0.1:1234/path", 0, 0)
	d.store = storeMock

	expected := discovery.Entries{
		&discovery.Entry{Host: "1.1.1.1", Port: "1111"},
		&discovery.Entry{Host: "2.2.2.2", Port: "2222"},
	}
	kvs := []*store.KVPair{
		{Key: path.Join("path", discoveryPath, "1.1.1.1"), Value: []byte("1.1.1.1:1111")},
		{Key: path.Join("path", discoveryPath, "2.2.2.2"), Value: []byte("2.2.2.2:2222")},
	}

	stopCh := make(chan struct{})
	ch, errCh := d.Watch(stopCh)

	// It should fire an error since the first WatchTree call failed.
	c.Assert(<-errCh, check.ErrorMatches, "test error")
	// We have to drain the error channel otherwise Watch will get stuck.
	go func() {
		for range errCh {
		}
	}()

	// Push the entries into the store channel and make sure discovery emits.
	mockCh <- kvs
	c.Assert(<-ch, check.DeepEquals, expected)

	// Add a new entry.
	expected = append(expected, &discovery.Entry{Host: "3.3.3.3", Port: "3333"})
	kvs = append(kvs, &store.KVPair{Key: path.Join("path", discoveryPath, "3.3.3.3"), Value: []byte("3.3.3.3:3333")})
	mockCh <- kvs
	c.Assert(<-ch, check.DeepEquals, expected)

	close(mockCh)
	// Give it enough time to call WatchTree.
	time.Sleep(3)

	// Stop and make sure it closes all channels.
	close(stopCh)
	c.Assert(<-ch, check.IsNil)
	c.Assert(<-errCh, check.IsNil)
}

// FakeStore implements store.Store methods. It mocks all store
// function in a simple, naive way.
type FakeStore struct {
	Endpoints  []string
	Options    *store.Config
	mockKVChan <-chan []*store.KVPair

	watchTreeCallCount int
}

func (s *FakeStore) Put(key string, value []byte, options *store.WriteOptions) error {
	return nil
}

func (s *FakeStore) Get(key string) (*store.KVPair, error) {
	return nil, nil
}

func (s *FakeStore) Delete(key string) error {
	return nil
}

func (s *FakeStore) Exists(key string) (bool, error) {
	return true, nil
}

func (s *FakeStore) Watch(key string, stopCh <-chan struct{}) (<-chan *store.KVPair, error) {
	return nil, nil
}

// WatchTree will fail the first time, and return the mockKVchan afterwards.
// This is the behaviour we need for testing.. If we need 'moar', should update this.
func (s *FakeStore) WatchTree(directory string, stopCh <-chan struct{}) (<-chan []*store.KVPair, error) {
	if s.watchTreeCallCount == 0 {
		s.watchTreeCallCount = 1
		return nil, errors.New("test error")
	}
	// First calls error
	return s.mockKVChan, nil
}

func (s *FakeStore) NewLock(key string, options *store.LockOptions) (store.Locker, error) {
	return nil, nil
}

func (s *FakeStore) List(directory string) ([]*store.KVPair, error) {
	return []*store.KVPair{}, nil
}

func (s *FakeStore) DeleteTree(directory string) error {
	return nil
}

func (s *FakeStore) AtomicPut(key string, value []byte, previous *store.KVPair, options *store.WriteOptions) (bool, *store.KVPair, error) {
	return true, nil, nil
}

func (s *FakeStore) AtomicDelete(key string, previous *store.KVPair) (bool, error) {
	return true, nil
}

func (s *FakeStore) Close() {
}

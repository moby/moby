package kv

import (
	"errors"
	"path"
	"testing"
	"time"

	"github.com/docker/docker/pkg/discovery"
	"github.com/docker/libkv/store"
	libkvmock "github.com/docker/libkv/store/mock"
	"github.com/stretchr/testify/mock"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DiscoverySuite struct{}

var _ = check.Suite(&DiscoverySuite{})

func (ds *DiscoverySuite) TestInitialize(c *check.C) {
	storeMock, err := libkvmock.New([]string{"127.0.0.1"}, nil)
	c.Assert(storeMock, check.NotNil)
	c.Assert(err, check.IsNil)

	d := &Discovery{backend: store.CONSUL}
	d.Initialize("127.0.0.1", 0, 0)
	d.store = storeMock

	s := d.store.(*libkvmock.Mock)
	c.Assert(s.Endpoints, check.HasLen, 1)
	c.Assert(s.Endpoints[0], check.Equals, "127.0.0.1")
	c.Assert(d.path, check.Equals, discoveryPath)

	storeMock, err = libkvmock.New([]string{"127.0.0.1:1234"}, nil)
	c.Assert(storeMock, check.NotNil)
	c.Assert(err, check.IsNil)

	d = &Discovery{backend: store.CONSUL}
	d.Initialize("127.0.0.1:1234/path", 0, 0)
	d.store = storeMock

	s = d.store.(*libkvmock.Mock)
	c.Assert(s.Endpoints, check.HasLen, 1)
	c.Assert(s.Endpoints[0], check.Equals, "127.0.0.1:1234")
	c.Assert(d.path, check.Equals, "path/"+discoveryPath)

	storeMock, err = libkvmock.New([]string{"127.0.0.1:1234", "127.0.0.2:1234", "127.0.0.3:1234"}, nil)
	c.Assert(storeMock, check.NotNil)
	c.Assert(err, check.IsNil)

	d = &Discovery{backend: store.CONSUL}
	d.Initialize("127.0.0.1:1234,127.0.0.2:1234,127.0.0.3:1234/path", 0, 0)
	d.store = storeMock

	s = d.store.(*libkvmock.Mock)
	c.Assert(s.Endpoints, check.HasLen, 3)
	c.Assert(s.Endpoints[0], check.Equals, "127.0.0.1:1234")
	c.Assert(s.Endpoints[1], check.Equals, "127.0.0.2:1234")
	c.Assert(s.Endpoints[2], check.Equals, "127.0.0.3:1234")

	c.Assert(d.path, check.Equals, "path/"+discoveryPath)
}

func (ds *DiscoverySuite) TestWatch(c *check.C) {
	storeMock, err := libkvmock.New([]string{"127.0.0.1:1234"}, nil)
	c.Assert(storeMock, check.NotNil)
	c.Assert(err, check.IsNil)

	d := &Discovery{backend: store.CONSUL}
	d.Initialize("127.0.0.1:1234/path", 0, 0)
	d.store = storeMock

	s := d.store.(*libkvmock.Mock)
	mockCh := make(chan []*store.KVPair)

	// The first watch will fail.
	s.On("WatchTree", "path/"+discoveryPath, mock.Anything).Return(mockCh, errors.New("test error")).Once()
	// The second one will succeed.
	s.On("WatchTree", "path/"+discoveryPath, mock.Anything).Return(mockCh, nil).Once()
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

	// Make sure that if an error occurs it retries.
	// This third call to WatchTree will be checked later by AssertExpectations.
	s.On("WatchTree", "path/"+discoveryPath, mock.Anything).Return(mockCh, nil)
	close(mockCh)
	// Give it enough time to call WatchTree.
	time.Sleep(3)

	// Stop and make sure it closes all channels.
	close(stopCh)
	c.Assert(<-ch, check.IsNil)
	c.Assert(<-errCh, check.IsNil)

	s.AssertExpectations(c)
}

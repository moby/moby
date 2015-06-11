package kv

import (
	"errors"
	"path"
	"testing"
	"time"

	"github.com/docker/swarm/discovery"
	"github.com/docker/swarm/pkg/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestInitialize(t *testing.T) {
	d := &Discovery{backend: store.MOCK}
	assert.NoError(t, d.Initialize("127.0.0.1", 0, 0))
	s := d.store.(*store.Mock)
	assert.Len(t, s.Endpoints, 1)
	assert.Equal(t, s.Endpoints[0], "127.0.0.1")
	assert.Equal(t, d.path, discoveryPath)

	d = &Discovery{backend: store.MOCK}
	assert.NoError(t, d.Initialize("127.0.0.1:1234/path", 0, 0))
	s = d.store.(*store.Mock)
	assert.Len(t, s.Endpoints, 1)
	assert.Equal(t, s.Endpoints[0], "127.0.0.1:1234")
	assert.Equal(t, d.path, "path/"+discoveryPath)

	d = &Discovery{backend: store.MOCK}
	assert.NoError(t, d.Initialize("127.0.0.1:1234,127.0.0.2:1234,127.0.0.3:1234/path", 0, 0))
	s = d.store.(*store.Mock)
	assert.Len(t, s.Endpoints, 3)
	assert.Equal(t, s.Endpoints[0], "127.0.0.1:1234")
	assert.Equal(t, s.Endpoints[1], "127.0.0.2:1234")
	assert.Equal(t, s.Endpoints[2], "127.0.0.3:1234")
	assert.Equal(t, d.path, "path/"+discoveryPath)
}

func TestWatch(t *testing.T) {
	d := &Discovery{backend: store.MOCK}
	assert.NoError(t, d.Initialize("127.0.0.1:1234/path", 0, 0))
	s := d.store.(*store.Mock)

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

	// It should fire an error since the first WatchRange call failed.
	assert.EqualError(t, <-errCh, "test error")
	// We have to drain the error channel otherwise Watch will get stuck.
	go func() {
		for _ = range errCh {
		}
	}()

	// Push the entries into the store channel and make sure discovery emits.
	mockCh <- kvs
	assert.Equal(t, <-ch, expected)

	// Add a new entry.
	expected = append(expected, &discovery.Entry{Host: "3.3.3.3", Port: "3333"})
	kvs = append(kvs, &store.KVPair{Key: path.Join("path", discoveryPath, "3.3.3.3"), Value: []byte("3.3.3.3:3333")})
	mockCh <- kvs
	assert.Equal(t, <-ch, expected)

	// Make sure that if an error occurs it retries.
	// This third call to WatchTree will be checked later by AssertExpectations.
	s.On("WatchTree", "path/"+discoveryPath, mock.Anything).Return(mockCh, nil)
	close(mockCh)
	// Give it enough time to call WatchTree.
	time.Sleep(3)

	// Stop and make sure it closes all channels.
	close(stopCh)
	assert.Nil(t, <-ch)
	assert.Nil(t, <-errCh)

	s.AssertExpectations(t)
}

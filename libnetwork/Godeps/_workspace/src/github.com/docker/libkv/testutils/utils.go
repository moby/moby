package testutils

import (
	"testing"
	"time"

	"github.com/docker/libkv/store"
	"github.com/stretchr/testify/assert"
)

// RunTestStore is an helper testing method that is
// called by each K/V backend sub-package testing
func RunTestStore(t *testing.T, kv store.Store, backup store.Store) {
	testPutGetDelete(t, kv)
	testWatch(t, kv)
	testWatchTree(t, kv)
	testAtomicPut(t, kv)
	testAtomicPutCreate(t, kv)
	testAtomicDelete(t, kv)
	testLockUnlock(t, kv)
	testPutEphemeral(t, kv, backup)
	testList(t, kv)
	testDeleteTree(t, kv)
}

func testPutGetDelete(t *testing.T, kv store.Store) {
	key := "foo"
	value := []byte("bar")

	// Put the key
	err := kv.Put(key, value, nil)
	assert.NoError(t, err)

	// Get should return the value and an incremented index
	pair, err := kv.Get(key)
	assert.NoError(t, err)
	if assert.NotNil(t, pair) {
		assert.NotNil(t, pair.Value)
	}
	assert.Equal(t, pair.Value, value)
	assert.NotEqual(t, pair.LastIndex, 0)

	// Delete the key
	err = kv.Delete(key)
	assert.NoError(t, err)

	// Get should fail
	pair, err = kv.Get(key)
	assert.Error(t, err)
	assert.Nil(t, pair)
}

func testWatch(t *testing.T, kv store.Store) {
	key := "hello"
	value := []byte("world")
	newValue := []byte("world!")

	// Put the key
	err := kv.Put(key, value, nil)
	assert.NoError(t, err)

	stopCh := make(<-chan struct{})
	events, err := kv.Watch(key, stopCh)
	assert.NoError(t, err)
	assert.NotNil(t, events)

	// Update loop
	go func() {
		timeout := time.After(1 * time.Second)
		tick := time.Tick(250 * time.Millisecond)
		for {
			select {
			case <-timeout:
				return
			case <-tick:
				err := kv.Put(key, newValue, nil)
				if assert.NoError(t, err) {
					continue
				}
				return
			}
		}
	}()

	// Check for updates
	eventCount := 1
	for {
		select {
		case event := <-events:
			assert.NotNil(t, event)
			if eventCount == 1 {
				assert.Equal(t, event.Key, key)
				assert.Equal(t, event.Value, value)
			} else {
				assert.Equal(t, event.Key, key)
				assert.Equal(t, event.Value, newValue)
			}
			eventCount++
			// We received all the events we wanted to check
			if eventCount >= 4 {
				return
			}
		case <-time.After(4 * time.Second):
			t.Fatal("Timeout reached")
			return
		}
	}
}

func testWatchTree(t *testing.T, kv store.Store) {
	dir := "tree"

	node1 := "tree/node1"
	value1 := []byte("node1")

	node2 := "tree/node2"
	value2 := []byte("node2")

	node3 := "tree/node3"
	value3 := []byte("node3")

	err := kv.Put(node1, value1, nil)
	assert.NoError(t, err)
	err = kv.Put(node2, value2, nil)
	assert.NoError(t, err)
	err = kv.Put(node3, value3, nil)
	assert.NoError(t, err)

	stopCh := make(<-chan struct{})
	events, err := kv.WatchTree(dir, stopCh)
	assert.NoError(t, err)
	assert.NotNil(t, events)

	// Update loop
	go func() {
		timeout := time.After(250 * time.Millisecond)
		for {
			select {
			case <-timeout:
				err := kv.Delete(node3)
				assert.NoError(t, err)
				return
			}
		}
	}()

	// Check for updates
	for {
		select {
		case event := <-events:
			assert.NotNil(t, event)
			// We received the Delete event on a child node
			// Exit test successfully
			if len(event) == 2 {
				return
			}
		case <-time.After(4 * time.Second):
			t.Fatal("Timeout reached")
			return
		}
	}
}

func testAtomicPut(t *testing.T, kv store.Store) {
	key := "hello"
	value := []byte("world")

	// Put the key
	err := kv.Put(key, value, nil)
	assert.NoError(t, err)

	// Get should return the value and an incremented index
	pair, err := kv.Get(key)
	assert.NoError(t, err)
	if assert.NotNil(t, pair) {
		assert.NotNil(t, pair.Value)
	}
	assert.Equal(t, pair.Value, value)
	assert.NotEqual(t, pair.LastIndex, 0)

	// This CAS should fail: previous exists.
	success, _, err := kv.AtomicPut("hello", []byte("WORLD"), nil, nil)
	assert.Error(t, err)
	assert.False(t, success)

	// This CAS should succeed
	success, _, err = kv.AtomicPut("hello", []byte("WORLD"), pair, nil)
	assert.NoError(t, err)
	assert.True(t, success)

	// This CAS should fail, key exists.
	pair.LastIndex = 0
	success, _, err = kv.AtomicPut("hello", []byte("WORLDWORLD"), pair, nil)
	assert.Error(t, err)
	assert.False(t, success)
}

func testAtomicPutCreate(t *testing.T, kv store.Store) {
	// Use a key in a new directory to ensure Stores will create directories
	// that don't yet exist.
	key := "put/create"
	value := []byte("putcreate")

	// AtomicPut the key, previous = nil indicates create.
	success, _, err := kv.AtomicPut(key, value, nil, nil)
	assert.NoError(t, err)
	assert.True(t, success)

	// Get should return the value and an incremented index
	pair, err := kv.Get(key)
	assert.NoError(t, err)
	if assert.NotNil(t, pair) {
		assert.NotNil(t, pair.Value)
	}
	assert.Equal(t, pair.Value, value)

	// Attempting to create again should fail.
	success, _, err = kv.AtomicPut(key, value, nil, nil)
	assert.Error(t, err)
	assert.False(t, success)

	// This CAS should succeed, since it has the value from Get()
	success, _, err = kv.AtomicPut(key, []byte("PUTCREATE"), pair, nil)
	assert.NoError(t, err)
	assert.True(t, success)

	// Delete the key, ensures runs of the test don't interfere with each other.
	err = kv.DeleteTree("put")
	assert.NoError(t, err)
}

func testAtomicDelete(t *testing.T, kv store.Store) {
	key := "atomic"
	value := []byte("world")

	// Put the key
	err := kv.Put(key, value, nil)
	assert.NoError(t, err)

	// Get should return the value and an incremented index
	pair, err := kv.Get(key)
	assert.NoError(t, err)
	if assert.NotNil(t, pair) {
		assert.NotNil(t, pair.Value)
	}
	assert.Equal(t, pair.Value, value)
	assert.NotEqual(t, pair.LastIndex, 0)

	tempIndex := pair.LastIndex

	// AtomicDelete should fail
	pair.LastIndex = 0
	success, err := kv.AtomicDelete(key, pair)
	assert.Error(t, err)
	assert.False(t, success)

	// AtomicDelete should succeed
	pair.LastIndex = tempIndex
	success, err = kv.AtomicDelete(key, pair)
	assert.NoError(t, err)
	assert.True(t, success)
}

func testLockUnlock(t *testing.T, kv store.Store) {
	key := "foo"
	value := []byte("bar")

	// We should be able to create a new lock on key
	lock, err := kv.NewLock(key, &store.LockOptions{Value: value})
	assert.NoError(t, err)
	assert.NotNil(t, lock)

	// Lock should successfully succeed or block
	lockChan, err := lock.Lock()
	assert.NoError(t, err)
	assert.NotNil(t, lockChan)

	// Get should work
	pair, err := kv.Get(key)
	assert.NoError(t, err)
	if assert.NotNil(t, pair) {
		assert.NotNil(t, pair.Value)
	}
	assert.Equal(t, pair.Value, value)
	assert.NotEqual(t, pair.LastIndex, 0)

	// Unlock should succeed
	err = lock.Unlock()
	assert.NoError(t, err)

	// Get should work
	pair, err = kv.Get(key)
	assert.NoError(t, err)
	if assert.NotNil(t, pair) {
		assert.NotNil(t, pair.Value)
	}
	assert.Equal(t, pair.Value, value)
	assert.NotEqual(t, pair.LastIndex, 0)
}

func testPutEphemeral(t *testing.T, kv store.Store, otherConn store.Store) {
	firstKey := "first"
	firstValue := []byte("foo")

	secondKey := "second"
	secondValue := []byte("bar")

	// Put the first key with the Ephemeral flag
	err := otherConn.Put(firstKey, firstValue, &store.WriteOptions{Ephemeral: true})
	assert.NoError(t, err)

	// Put a second key with the Ephemeral flag
	err = otherConn.Put(secondKey, secondValue, &store.WriteOptions{Ephemeral: true})
	assert.NoError(t, err)

	// Get on firstKey should work
	pair, err := kv.Get(firstKey)
	assert.NoError(t, err)
	assert.NotNil(t, pair)

	// Get on secondKey should work
	pair, err = kv.Get(secondKey)
	assert.NoError(t, err)
	assert.NotNil(t, pair)

	// Close the connection
	otherConn.Close()

	// Let the session expire
	time.Sleep(3 * time.Second)

	// Get on firstKey shouldn't work
	pair, err = kv.Get(firstKey)
	assert.Error(t, err)
	assert.Nil(t, pair)

	// Get on secondKey shouldn't work
	pair, err = kv.Get(secondKey)
	assert.Error(t, err)
	assert.Nil(t, pair)
}

func testList(t *testing.T, kv store.Store) {
	prefix := "nodes"

	firstKey := "nodes/first"
	firstValue := []byte("first")

	secondKey := "nodes/second"
	secondValue := []byte("second")

	// Put the first key
	err := kv.Put(firstKey, firstValue, nil)
	assert.NoError(t, err)

	// Put the second key
	err = kv.Put(secondKey, secondValue, nil)
	assert.NoError(t, err)

	// List should work and return the two correct values
	pairs, err := kv.List(prefix)
	assert.NoError(t, err)
	if assert.NotNil(t, pairs) {
		assert.Equal(t, len(pairs), 2)
	}

	// Check pairs, those are not necessarily in Put order
	for _, pair := range pairs {
		if pair.Key == firstKey {
			assert.Equal(t, pair.Value, firstValue)
		}
		if pair.Key == secondKey {
			assert.Equal(t, pair.Value, secondValue)
		}
	}

	// List should fail: the key does not exist
	pairs, err = kv.List("idontexist")
	assert.Error(t, err)
	assert.Nil(t, pairs)
}

func testDeleteTree(t *testing.T, kv store.Store) {
	prefix := "nodes"

	firstKey := "nodes/first"
	firstValue := []byte("first")

	secondKey := "nodes/second"
	secondValue := []byte("second")

	// Put the first key
	err := kv.Put(firstKey, firstValue, nil)
	assert.NoError(t, err)

	// Put the second key
	err = kv.Put(secondKey, secondValue, nil)
	assert.NoError(t, err)

	// Get should work on the first Key
	pair, err := kv.Get(firstKey)
	assert.NoError(t, err)
	if assert.NotNil(t, pair) {
		assert.NotNil(t, pair.Value)
	}
	assert.Equal(t, pair.Value, firstValue)
	assert.NotEqual(t, pair.LastIndex, 0)

	// Get should work on the second Key
	pair, err = kv.Get(secondKey)
	assert.NoError(t, err)
	if assert.NotNil(t, pair) {
		assert.NotNil(t, pair.Value)
	}
	assert.Equal(t, pair.Value, secondValue)
	assert.NotEqual(t, pair.LastIndex, 0)

	// Delete Values under directory `nodes`
	err = kv.DeleteTree(prefix)
	assert.NoError(t, err)

	// Get should fail on both keys
	pair, err = kv.Get(firstKey)
	assert.Error(t, err)
	assert.Nil(t, pair)

	pair, err = kv.Get(secondKey)
	assert.Error(t, err)
	assert.Nil(t, pair)
}

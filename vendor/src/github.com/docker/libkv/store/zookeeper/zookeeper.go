package zookeeper

import (
	"strings"
	"time"

	"github.com/docker/libkv"
	"github.com/docker/libkv/store"
	zk "github.com/samuel/go-zookeeper/zk"
)

const (
	// SOH control character
	SOH = "\x01"

	defaultTimeout = 10 * time.Second
)

// Zookeeper is the receiver type for
// the Store interface
type Zookeeper struct {
	timeout time.Duration
	client  *zk.Conn
}

type zookeeperLock struct {
	client *zk.Conn
	lock   *zk.Lock
	key    string
	value  []byte
}

// Register registers zookeeper to libkv
func Register() {
	libkv.AddStore(store.ZK, New)
}

// New creates a new Zookeeper client given a
// list of endpoints and an optional tls config
func New(endpoints []string, options *store.Config) (store.Store, error) {
	s := &Zookeeper{}
	s.timeout = defaultTimeout

	// Set options
	if options != nil {
		if options.ConnectionTimeout != 0 {
			s.setTimeout(options.ConnectionTimeout)
		}
	}

	// Connect to Zookeeper
	conn, _, err := zk.Connect(endpoints, s.timeout)
	if err != nil {
		return nil, err
	}
	s.client = conn

	return s, nil
}

// setTimeout sets the timeout for connecting to Zookeeper
func (s *Zookeeper) setTimeout(time time.Duration) {
	s.timeout = time
}

// Get the value at "key", returns the last modified index
// to use in conjunction to Atomic calls
func (s *Zookeeper) Get(key string) (pair *store.KVPair, err error) {
	resp, meta, err := s.client.Get(s.normalize(key))

	if err != nil {
		if err == zk.ErrNoNode {
			return nil, store.ErrKeyNotFound
		}
		return nil, err
	}

	// FIXME handle very rare cases where Get returns the
	// SOH control character instead of the actual value
	if string(resp) == SOH {
		return s.Get(store.Normalize(key))
	}

	pair = &store.KVPair{
		Key:       key,
		Value:     resp,
		LastIndex: uint64(meta.Version),
	}

	return pair, nil
}

// createFullPath creates the entire path for a directory
// that does not exist
func (s *Zookeeper) createFullPath(path []string, ephemeral bool) error {
	for i := 1; i <= len(path); i++ {
		newpath := "/" + strings.Join(path[:i], "/")
		if i == len(path) && ephemeral {
			_, err := s.client.Create(newpath, []byte{}, zk.FlagEphemeral, zk.WorldACL(zk.PermAll))
			return err
		}
		_, err := s.client.Create(newpath, []byte{}, 0, zk.WorldACL(zk.PermAll))
		if err != nil {
			// Skip if node already exists
			if err != zk.ErrNodeExists {
				return err
			}
		}
	}
	return nil
}

// Put a value at "key"
func (s *Zookeeper) Put(key string, value []byte, opts *store.WriteOptions) error {
	fkey := s.normalize(key)

	exists, err := s.Exists(key)
	if err != nil {
		return err
	}

	if !exists {
		if opts != nil && opts.TTL > 0 {
			s.createFullPath(store.SplitKey(strings.TrimSuffix(key, "/")), true)
		} else {
			s.createFullPath(store.SplitKey(strings.TrimSuffix(key, "/")), false)
		}
	}

	_, err = s.client.Set(fkey, value, -1)
	return err
}

// Delete a value at "key"
func (s *Zookeeper) Delete(key string) error {
	err := s.client.Delete(s.normalize(key), -1)
	if err == zk.ErrNoNode {
		return store.ErrKeyNotFound
	}
	return err
}

// Exists checks if the key exists inside the store
func (s *Zookeeper) Exists(key string) (bool, error) {
	exists, _, err := s.client.Exists(s.normalize(key))
	if err != nil {
		return false, err
	}
	return exists, nil
}

// Watch for changes on a "key"
// It returns a channel that will receive changes or pass
// on errors. Upon creation, the current value will first
// be sent to the channel. Providing a non-nil stopCh can
// be used to stop watching.
func (s *Zookeeper) Watch(key string, stopCh <-chan struct{}) (<-chan *store.KVPair, error) {
	// Get the key first
	pair, err := s.Get(key)
	if err != nil {
		return nil, err
	}

	// Catch zk notifications and fire changes into the channel.
	watchCh := make(chan *store.KVPair)
	go func() {
		defer close(watchCh)

		// Get returns the current value to the channel prior
		// to listening to any event that may occur on that key
		watchCh <- pair
		for {
			_, _, eventCh, err := s.client.GetW(s.normalize(key))
			if err != nil {
				return
			}
			select {
			case e := <-eventCh:
				if e.Type == zk.EventNodeDataChanged {
					if entry, err := s.Get(key); err == nil {
						watchCh <- entry
					}
				}
			case <-stopCh:
				// There is no way to stop GetW so just quit
				return
			}
		}
	}()

	return watchCh, nil
}

// WatchTree watches for changes on a "directory"
// It returns a channel that will receive changes or pass
// on errors. Upon creating a watch, the current childs values
// will be sent to the channel .Providing a non-nil stopCh can
// be used to stop watching.
func (s *Zookeeper) WatchTree(directory string, stopCh <-chan struct{}) (<-chan []*store.KVPair, error) {
	// List the childrens first
	entries, err := s.List(directory)
	if err != nil {
		return nil, err
	}

	// Catch zk notifications and fire changes into the channel.
	watchCh := make(chan []*store.KVPair)
	go func() {
		defer close(watchCh)

		// List returns the children values to the channel
		// prior to listening to any events that may occur
		// on those keys
		watchCh <- entries

		for {
			_, _, eventCh, err := s.client.ChildrenW(s.normalize(directory))
			if err != nil {
				return
			}
			select {
			case e := <-eventCh:
				if e.Type == zk.EventNodeChildrenChanged {
					if kv, err := s.List(directory); err == nil {
						watchCh <- kv
					}
				}
			case <-stopCh:
				// There is no way to stop GetW so just quit
				return
			}
		}
	}()

	return watchCh, nil
}

// List child nodes of a given directory
func (s *Zookeeper) List(directory string) ([]*store.KVPair, error) {
	keys, stat, err := s.client.Children(s.normalize(directory))
	if err != nil {
		if err == zk.ErrNoNode {
			return nil, store.ErrKeyNotFound
		}
		return nil, err
	}

	kv := []*store.KVPair{}

	// FIXME Costly Get request for each child key..
	for _, key := range keys {
		pair, err := s.Get(strings.TrimSuffix(directory, "/") + s.normalize(key))
		if err != nil {
			// If node is not found: List is out of date, retry
			if err == zk.ErrNoNode {
				return s.List(directory)
			}
			return nil, err
		}

		kv = append(kv, &store.KVPair{
			Key:       key,
			Value:     []byte(pair.Value),
			LastIndex: uint64(stat.Version),
		})
	}

	return kv, nil
}

// DeleteTree deletes a range of keys under a given directory
func (s *Zookeeper) DeleteTree(directory string) error {
	pairs, err := s.List(directory)
	if err != nil {
		return err
	}

	var reqs []interface{}

	for _, pair := range pairs {
		reqs = append(reqs, &zk.DeleteRequest{
			Path:    s.normalize(directory + "/" + pair.Key),
			Version: -1,
		})
	}

	_, err = s.client.Multi(reqs...)
	return err
}

// AtomicPut put a value at "key" if the key has not been
// modified in the meantime, throws an error if this is the case
func (s *Zookeeper) AtomicPut(key string, value []byte, previous *store.KVPair, _ *store.WriteOptions) (bool, *store.KVPair, error) {

	var lastIndex uint64
	if previous != nil {
		meta, err := s.client.Set(s.normalize(key), value, int32(previous.LastIndex))
		if err != nil {
			// Compare Failed
			if err == zk.ErrBadVersion {
				return false, nil, store.ErrKeyModified
			}
			return false, nil, err
		}
		lastIndex = uint64(meta.Version)
	} else {
		// Interpret previous == nil as create operation.
		_, err := s.client.Create(s.normalize(key), value, 0, zk.WorldACL(zk.PermAll))
		if err != nil {
			// Zookeeper will complain if the directory doesn't exist.
			if err == zk.ErrNoNode {
				// Create the directory
				parts := store.SplitKey(strings.TrimSuffix(key, "/"))
				parts = parts[:len(parts)-1]
				if err = s.createFullPath(parts, false); err != nil {
					// Failed to create the directory.
					return false, nil, err
				}
				if _, err := s.client.Create(s.normalize(key), value, 0, zk.WorldACL(zk.PermAll)); err != nil {
					return false, nil, err
				}

			} else {
				// Unhandled error
				return false, nil, err
			}
		}
		lastIndex = 0 // Newly created nodes have version 0.
	}

	pair := &store.KVPair{
		Key:       key,
		Value:     value,
		LastIndex: lastIndex,
	}

	return true, pair, nil
}

// AtomicDelete deletes a value at "key" if the key
// has not been modified in the meantime, throws an
// error if this is the case
func (s *Zookeeper) AtomicDelete(key string, previous *store.KVPair) (bool, error) {
	if previous == nil {
		return false, store.ErrPreviousNotSpecified
	}

	err := s.client.Delete(s.normalize(key), int32(previous.LastIndex))
	if err != nil {
		// Key not found
		if err == zk.ErrNoNode {
			return false, store.ErrKeyNotFound
		}
		// Compare failed
		if err == zk.ErrBadVersion {
			return false, store.ErrKeyModified
		}
		// General store error
		return false, err
	}
	return true, nil
}

// NewLock returns a handle to a lock struct which can
// be used to provide mutual exclusion on a key
func (s *Zookeeper) NewLock(key string, options *store.LockOptions) (lock store.Locker, err error) {
	value := []byte("")

	// Apply options
	if options != nil {
		if options.Value != nil {
			value = options.Value
		}
	}

	lock = &zookeeperLock{
		client: s.client,
		key:    s.normalize(key),
		value:  value,
		lock:   zk.NewLock(s.client, s.normalize(key), zk.WorldACL(zk.PermAll)),
	}

	return lock, err
}

// Lock attempts to acquire the lock and blocks while
// doing so. It returns a channel that is closed if our
// lock is lost or if an error occurs
func (l *zookeeperLock) Lock(stopChan chan struct{}) (<-chan struct{}, error) {
	err := l.lock.Lock()

	if err == nil {
		// We hold the lock, we can set our value
		// FIXME: The value is left behind
		// (problematic for leader election)
		_, err = l.client.Set(l.key, l.value, -1)
	}

	return make(chan struct{}), err
}

// Unlock the "key". Calling unlock while
// not holding the lock will throw an error
func (l *zookeeperLock) Unlock() error {
	return l.lock.Unlock()
}

// Close closes the client connection
func (s *Zookeeper) Close() {
	s.client.Close()
}

// Normalize the key for usage in Zookeeper
func (s *Zookeeper) normalize(key string) string {
	key = store.Normalize(key)
	return strings.TrimSuffix(key, "/")
}

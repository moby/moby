package store

import (
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	zk "github.com/samuel/go-zookeeper/zk"
)

const defaultTimeout = 10 * time.Second

// Zookeeper embeds the zookeeper client
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

// InitializeZookeeper creates a new Zookeeper client
// given a list of endpoints and optional tls config
func InitializeZookeeper(endpoints []string, options *Config) (Store, error) {
	s := &Zookeeper{}
	s.timeout = defaultTimeout

	// Set options
	if options != nil {
		if options.ConnectionTimeout != 0 {
			s.setTimeout(options.ConnectionTimeout)
		}
	}

	conn, _, err := zk.Connect(endpoints, s.timeout)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	s.client = conn
	return s, nil
}

// SetTimeout sets the timout for connecting to Zookeeper
func (s *Zookeeper) setTimeout(time time.Duration) {
	s.timeout = time
}

// Get the value at "key", returns the last modified index
// to use in conjunction to CAS calls
func (s *Zookeeper) Get(key string) (*KVPair, error) {
	resp, meta, err := s.client.Get(normalize(key))
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, ErrKeyNotFound
	}
	return &KVPair{key, resp, uint64(meta.Version)}, nil
}

// Create the entire path for a directory that does not exist
func (s *Zookeeper) createFullpath(path []string, ephemeral bool) error {
	for i := 1; i <= len(path); i++ {
		newpath := "/" + strings.Join(path[:i], "/")
		if i == len(path) && ephemeral {
			_, err := s.client.Create(newpath, []byte{1}, zk.FlagEphemeral, zk.WorldACL(zk.PermAll))
			return err
		}
		_, err := s.client.Create(newpath, []byte{1}, 0, zk.WorldACL(zk.PermAll))
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
func (s *Zookeeper) Put(key string, value []byte, opts *WriteOptions) error {
	fkey := normalize(key)
	exists, err := s.Exists(key)
	if err != nil {
		return err
	}
	if !exists {
		if opts != nil && opts.Ephemeral {
			s.createFullpath(splitKey(key), opts.Ephemeral)
		} else {
			s.createFullpath(splitKey(key), false)
		}
	}
	_, err = s.client.Set(fkey, value, -1)
	return err
}

// Delete a value at "key"
func (s *Zookeeper) Delete(key string) error {
	err := s.client.Delete(normalize(key), -1)
	return err
}

// Exists checks if the key exists inside the store
func (s *Zookeeper) Exists(key string) (bool, error) {
	exists, _, err := s.client.Exists(normalize(key))
	if err != nil {
		return false, err
	}
	return exists, nil
}

// Watch changes on a key.
// Returns a channel that will receive changes or an error.
// Upon creating a watch, the current value will be sent to the channel.
// Providing a non-nil stopCh can be used to stop watching.
func (s *Zookeeper) Watch(key string, stopCh <-chan struct{}) (<-chan *KVPair, error) {
	fkey := normalize(key)
	pair, err := s.Get(key)
	if err != nil {
		return nil, err
	}

	// Catch zk notifications and fire changes into the channel.
	watchCh := make(chan *KVPair)
	go func() {
		defer close(watchCh)

		// Get returns the current value before setting the watch.
		watchCh <- pair
		for {
			_, _, eventCh, err := s.client.GetW(fkey)
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

// WatchTree watches changes on a "directory"
// Returns a channel that will receive changes or an error.
// Upon creating a watch, the current value will be sent to the channel.
// Providing a non-nil stopCh can be used to stop watching.
func (s *Zookeeper) WatchTree(prefix string, stopCh <-chan struct{}) (<-chan []*KVPair, error) {
	fprefix := normalize(prefix)
	entries, err := s.List(prefix)
	if err != nil {
		return nil, err
	}

	// Catch zk notifications and fire changes into the channel.
	watchCh := make(chan []*KVPair)
	go func() {
		defer close(watchCh)

		// List returns the current values before setting the watch.
		watchCh <- entries

		for {
			_, _, eventCh, err := s.client.ChildrenW(fprefix)
			if err != nil {
				return
			}
			select {
			case e := <-eventCh:
				if e.Type == zk.EventNodeChildrenChanged {
					if kv, err := s.List(prefix); err == nil {
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

// List the content of a given prefix
func (s *Zookeeper) List(prefix string) ([]*KVPair, error) {
	keys, stat, err := s.client.Children(normalize(prefix))
	if err != nil {
		return nil, err
	}
	kv := []*KVPair{}
	for _, key := range keys {
		// FIXME Costly Get request for each child key..
		pair, err := s.Get(prefix + normalize(key))
		if err != nil {
			return nil, err
		}
		kv = append(kv, &KVPair{key, []byte(pair.Value), uint64(stat.Version)})
	}
	return kv, nil
}

// DeleteTree deletes a range of keys based on prefix
func (s *Zookeeper) DeleteTree(prefix string) error {
	pairs, err := s.List(prefix)
	if err != nil {
		return err
	}
	var reqs []interface{}
	for _, pair := range pairs {
		reqs = append(reqs, &zk.DeleteRequest{
			Path:    normalize(prefix + "/" + pair.Key),
			Version: -1,
		})
	}
	_, err = s.client.Multi(reqs...)
	return err
}

// AtomicPut put a value at "key" if the key has not been
// modified in the meantime, throws an error if this is the case
func (s *Zookeeper) AtomicPut(key string, value []byte, previous *KVPair, _ *WriteOptions) (bool, *KVPair, error) {
	if previous == nil {
		return false, nil, ErrPreviousNotSpecified
	}

	meta, err := s.client.Set(normalize(key), value, int32(previous.LastIndex))
	if err != nil {
		if err == zk.ErrBadVersion {
			return false, nil, ErrKeyModified
		}
		return false, nil, err
	}
	return true, &KVPair{Key: key, Value: value, LastIndex: uint64(meta.Version)}, nil
}

// AtomicDelete deletes a value at "key" if the key has not
// been modified in the meantime, throws an error if this is the case
func (s *Zookeeper) AtomicDelete(key string, previous *KVPair) (bool, error) {
	if previous == nil {
		return false, ErrPreviousNotSpecified
	}

	err := s.client.Delete(normalize(key), int32(previous.LastIndex))
	if err != nil {
		if err == zk.ErrBadVersion {
			return false, ErrKeyModified
		}
		return false, err
	}
	return true, nil
}

// NewLock returns a handle to a lock struct which can be used to acquire and
// release the mutex.
func (s *Zookeeper) NewLock(key string, options *LockOptions) (Locker, error) {
	value := []byte("")

	// Apply options
	if options != nil {
		if options.Value != nil {
			value = options.Value
		}
	}

	return &zookeeperLock{
		client: s.client,
		key:    normalize(key),
		value:  value,
		lock:   zk.NewLock(s.client, normalize(key), zk.WorldACL(zk.PermAll)),
	}, nil
}

// Lock attempts to acquire the lock and blocks while doing so.
// Returns a channel that is closed if our lock is lost or an error.
func (l *zookeeperLock) Lock() (<-chan struct{}, error) {
	err := l.lock.Lock()

	if err == nil {
		// We hold the lock, we can set our value
		// FIXME: When the last leader leaves the election, this value will be left behind
		_, err = l.client.Set(l.key, l.value, -1)
	}

	return make(chan struct{}), err
}

// Unlock released the lock. It is an error to call this
// if the lock is not currently held.
func (l *zookeeperLock) Unlock() error {
	return l.lock.Unlock()
}

// Close closes the client connection
func (s *Zookeeper) Close() {
	s.client.Close()
}

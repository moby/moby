package store

import (
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	zk "github.com/samuel/go-zookeeper/zk"
)

// Zookeeper embeds the zookeeper client
// and list of watches
type Zookeeper struct {
	timeout time.Duration
	client  *zk.Conn
	watches map[string]<-chan zk.Event
}

// InitializeZookeeper creates a new Zookeeper client
// given a list of endpoints and optional tls config
func InitializeZookeeper(endpoints []string, options Config) (Store, error) {
	s := &Zookeeper{}
	s.watches = make(map[string]<-chan zk.Event)
	s.timeout = 5 * time.Second // default timeout

	if options.Timeout != 0 {
		s.setTimeout(options.Timeout)
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
func (s *Zookeeper) Get(key string) (value []byte, lastIndex uint64, err error) {
	resp, meta, err := s.client.Get(format(key))
	if err != nil {
		return nil, 0, err
	}
	if resp == nil {
		return nil, 0, ErrKeyNotFound
	}
	return resp, uint64(meta.Mzxid), nil
}

// Create the entire path for a directory that does not exist
func (s *Zookeeper) createFullpath(path []string) error {
	for i := 1; i <= len(path); i++ {
		newpath := "/" + strings.Join(path[:i], "/")
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
func (s *Zookeeper) Put(key string, value []byte) error {
	fkey := format(key)
	exists, err := s.Exists(key)
	if err != nil {
		return err
	}
	if !exists {
		s.createFullpath(splitKey(key))
	}
	_, err = s.client.Set(fkey, value, -1)
	return err
}

// Delete a value at "key"
func (s *Zookeeper) Delete(key string) error {
	err := s.client.Delete(format(key), -1)
	return err
}

// Exists checks if the key exists inside the store
func (s *Zookeeper) Exists(key string) (bool, error) {
	exists, _, err := s.client.Exists(format(key))
	if err != nil {
		return false, err
	}
	return exists, nil
}

// Watch a single key for modifications
func (s *Zookeeper) Watch(key string, _ time.Duration, callback WatchCallback) error {
	fkey := format(key)
	_, _, eventChan, err := s.client.GetW(fkey)
	if err != nil {
		return err
	}

	// Create a new Watch entry with eventChan
	s.watches[fkey] = eventChan

	for e := range eventChan {
		if e.Type == zk.EventNodeChildrenChanged {
			log.WithField("name", "zk").Debug("Discovery watch triggered")
			entry, index, err := s.Get(key)
			kvi := []KVEntry{&kviTuple{key, []byte(entry), index}}
			if err == nil {
				callback(kvi)
			}
		}
	}

	return nil
}

// CancelWatch cancels a watch, sends a signal to the appropriate
// stop channel
func (s *Zookeeper) CancelWatch(key string) error {
	key = format(key)
	if _, ok := s.watches[key]; !ok {
		log.Error("Chan does not exist for key: ", key)
		return ErrWatchDoesNotExist
	}
	// Just remove the entry on watches key
	s.watches[key] = nil
	return nil
}

// GetRange gets a range of values at "directory"
func (s *Zookeeper) GetRange(prefix string) (kvi []KVEntry, err error) {
	prefix = format(prefix)
	entries, stat, err := s.client.Children(prefix)
	if err != nil {
		log.Error("Cannot fetch range of keys beginning with prefix: ", prefix)
		return nil, err
	}
	for _, item := range entries {
		kvi = append(kvi, &kviTuple{prefix, []byte(item), uint64(stat.Mzxid)})
	}
	return kvi, err
}

// DeleteRange deletes a range of values at "directory"
func (s *Zookeeper) DeleteRange(prefix string) error {
	err := s.client.Delete(format(prefix), -1)
	return err
}

// WatchRange triggers a watch on a range of values at "directory"
func (s *Zookeeper) WatchRange(prefix string, filter string, _ time.Duration, callback WatchCallback) error {
	fprefix := format(prefix)
	_, _, eventChan, err := s.client.ChildrenW(fprefix)
	if err != nil {
		return err
	}

	// Create a new Watch entry with eventChan
	s.watches[fprefix] = eventChan

	for e := range eventChan {
		if e.Type == zk.EventNodeChildrenChanged {
			log.WithField("name", "zk").Debug("Discovery watch triggered")
			kvi, err := s.GetRange(prefix)
			if err == nil {
				callback(kvi)
			}
		}
	}

	return nil
}

// CancelWatchRange stops the watch on the range of values, sends
// a signal to the appropriate stop channel
func (s *Zookeeper) CancelWatchRange(prefix string) error {
	return s.CancelWatch(prefix)
}

// AtomicPut put a value at "key" if the key has not been
// modified in the meantime, throws an error if this is the case
func (s *Zookeeper) AtomicPut(key string, oldValue []byte, newValue []byte, index uint64) (bool, error) {
	// Use index of Set method to implement CAS
	return false, ErrNotImplemented
}

// AtomicDelete deletes a value at "key" if the key has not
// been modified in the meantime, throws an error if this is the case
func (s *Zookeeper) AtomicDelete(key string, oldValue []byte, index uint64) (bool, error) {
	return false, ErrNotImplemented
}

// Acquire the lock for "key"/"directory"
func (s *Zookeeper) Acquire(path string, value []byte) (string, error) {
	// lock := zk.NewLock(s.client, path, nil)
	// locks[path] = lock
	// lock.Lock()
	return "", ErrNotImplemented
}

// Release the lock for "key"/"directory"
func (s *Zookeeper) Release(session string) error {
	return ErrNotImplemented
}

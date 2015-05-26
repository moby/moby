package store

import (
	"crypto/tls"
	"net"
	"net/http"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	etcd "github.com/coreos/go-etcd/etcd"
)

// Etcd embeds the client
type Etcd struct {
	client  *etcd.Client
	watches map[string]chan<- bool
}

// InitializeEtcd creates a new Etcd client given
// a list of endpoints and optional tls config
func InitializeEtcd(addrs []string, options Config) (Store, error) {
	s := &Etcd{}
	s.watches = make(map[string]chan<- bool)

	entries := createEndpoints(addrs, "http")
	s.client = etcd.NewClient(entries)

	if options.TLS != nil {
		s.setTLS(options.TLS)
	}

	if options.Timeout != 0 {
		s.setTimeout(options.Timeout)
	}

	return s, nil
}

// SetTLS sets the tls configuration given the path
// of certificate files
func (s *Etcd) setTLS(tls *tls.Config) {
	// Change to https scheme
	var addrs []string
	entries := s.client.GetCluster()
	for _, entry := range entries {
		addrs = append(addrs, strings.Replace(entry, "http", "https", -1))
	}
	s.client.SetCluster(addrs)

	// Set transport
	t := http.Transport{
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second, // default timeout
			KeepAlive: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     tls,
	}
	s.client.SetTransport(&t)
}

// SetTimeout sets the timeout used for connecting to the store
func (s *Etcd) setTimeout(time time.Duration) {
	s.client.SetDialTimeout(time)
}

// Create the entire path for a directory that does not exist
func (s *Etcd) createDirectory(path string) error {
	// TODO Handle TTL at key/dir creation -> use K/V struct for key infos?
	if _, err := s.client.CreateDir(format(path), 10); err != nil {
		if etcdError, ok := err.(*etcd.EtcdError); ok {
			if etcdError.ErrorCode != 105 { // Skip key already exists
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

// Get the value at "key", returns the last modified index
// to use in conjunction to CAS calls
func (s *Etcd) Get(key string) (value []byte, lastIndex uint64, err error) {
	result, err := s.client.Get(format(key), false, false)
	if err != nil {
		if etcdError, ok := err.(*etcd.EtcdError); ok {
			// Not a Directory or Not a file
			if etcdError.ErrorCode == 102 || etcdError.ErrorCode == 104 {
				return nil, 0, ErrKeyNotFound
			}
		}
		return nil, 0, err
	}
	return []byte(result.Node.Value), result.Node.ModifiedIndex, nil
}

// Put a value at "key"
func (s *Etcd) Put(key string, value []byte) error {
	if _, err := s.client.Set(key, string(value), 0); err != nil {
		if etcdError, ok := err.(*etcd.EtcdError); ok {
			if etcdError.ErrorCode == 104 { // Not a directory
				// Remove the last element (the actual key) and set the prefix as a dir
				err = s.createDirectory(getDir(key))
				if _, err := s.client.Set(key, string(value), 0); err != nil {
					return err
				}
			}
		}
		return err
	}
	return nil
}

// Delete a value at "key"
func (s *Etcd) Delete(key string) error {
	if _, err := s.client.Delete(format(key), false); err != nil {
		return err
	}
	return nil
}

// Exists checks if the key exists inside the store
func (s *Etcd) Exists(key string) (bool, error) {
	value, _, err := s.Get(key)
	if err != nil {
		if err == ErrKeyNotFound || value == nil {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Watch a single key for modifications
func (s *Etcd) Watch(key string, _ time.Duration, callback WatchCallback) error {
	key = format(key)
	watchChan := make(chan *etcd.Response)
	stopChan := make(chan bool)

	// Create new Watch entry
	s.watches[key] = stopChan

	// Start watch
	go s.client.Watch(key, 0, false, watchChan, stopChan)

	for _ = range watchChan {
		log.WithField("name", "etcd").Debug("Discovery watch triggered")
		entry, index, err := s.Get(key)
		if err != nil {
			log.Error("Cannot refresh the key: ", key, ", cancelling watch")
			s.watches[key] = nil
			return err
		}
		kvi := []KVEntry{&kviTuple{key, entry, index}}
		callback(kvi)
	}
	return nil
}

// CancelWatch cancels a watch, sends a signal to the appropriate
// stop channel
func (s *Etcd) CancelWatch(key string) error {
	key = format(key)
	if _, ok := s.watches[key]; !ok {
		log.Error("Chan does not exist for key: ", key)
		return ErrWatchDoesNotExist
	}
	// Send stop signal to event chan
	s.watches[key] <- true
	s.watches[key] = nil
	return nil
}

// AtomicPut put a value at "key" if the key has not been
// modified in the meantime, throws an error if this is the case
func (s *Etcd) AtomicPut(key string, oldValue []byte, newValue []byte, index uint64) (bool, error) {
	resp, err := s.client.CompareAndSwap(format(key), string(newValue), 5, string(oldValue), 0)
	if err != nil {
		return false, err
	}
	if !(resp.Node.Value == string(newValue) && resp.Node.Key == key && resp.Node.TTL == 5) {
		return false, ErrKeyModified
	}
	if !(resp.PrevNode.Value == string(newValue) && resp.PrevNode.Key == key && resp.PrevNode.TTL == 5) {
		return false, ErrKeyModified
	}
	return true, nil
}

// AtomicDelete deletes a value at "key" if the key has not
// been modified in the meantime, throws an error if this is the case
func (s *Etcd) AtomicDelete(key string, oldValue []byte, index uint64) (bool, error) {
	resp, err := s.client.CompareAndDelete(format(key), string(oldValue), 0)
	if err != nil {
		return false, err
	}
	if !(resp.PrevNode.Value == string(oldValue) && resp.PrevNode.Key == key && resp.PrevNode.TTL == 5) {
		return false, ErrKeyModified
	}
	return true, nil
}

// GetRange gets a range of values at "directory"
func (s *Etcd) GetRange(prefix string) ([]KVEntry, error) {
	resp, err := s.client.Get(format(prefix), true, true)
	if err != nil {
		return nil, err
	}
	kvi := make([]KVEntry, len(resp.Node.Nodes))
	for i, n := range resp.Node.Nodes {
		kvi[i] = &kviTuple{n.Key, []byte(n.Value), n.ModifiedIndex}
	}
	return kvi, nil
}

// DeleteRange deletes a range of values at "directory"
func (s *Etcd) DeleteRange(prefix string) error {
	if _, err := s.client.Delete(format(prefix), true); err != nil {
		return err
	}
	return nil
}

// WatchRange triggers a watch on a range of values at "directory"
func (s *Etcd) WatchRange(prefix string, filter string, _ time.Duration, callback WatchCallback) error {
	prefix = format(prefix)
	watchChan := make(chan *etcd.Response)
	stopChan := make(chan bool)

	// Create new Watch entry
	s.watches[prefix] = stopChan

	// Start watch
	go s.client.Watch(prefix, 0, true, watchChan, stopChan)
	for _ = range watchChan {
		log.WithField("name", "etcd").Debug("Discovery watch triggered")
		kvi, err := s.GetRange(prefix)
		if err != nil {
			log.Error("Cannot refresh the key: ", prefix, ", cancelling watch")
			s.watches[prefix] = nil
			return err
		}
		callback(kvi)
	}
	return nil
}

// CancelWatchRange stops the watch on the range of values, sends
// a signal to the appropriate stop channel
func (s *Etcd) CancelWatchRange(prefix string) error {
	return s.CancelWatch(format(prefix))
}

// Acquire the lock for "key"/"directory"
func (s *Etcd) Acquire(key string, value []byte) (string, error) {
	return "", ErrNotImplemented
}

// Release the lock for "key"/"directory"
func (s *Etcd) Release(session string) error {
	return ErrNotImplemented
}

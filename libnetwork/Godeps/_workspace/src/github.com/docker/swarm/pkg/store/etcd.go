package store

import (
	"crypto/tls"
	"net"
	"net/http"
	"strings"
	"time"

	etcd "github.com/coreos/go-etcd/etcd"
)

// Etcd embeds the client
type Etcd struct {
	client       *etcd.Client
	ephemeralTTL time.Duration
}

type etcdLock struct {
	client   *etcd.Client
	stopLock chan struct{}
	key      string
	value    string
	last     *etcd.Response
	ttl      uint64
}

const (
	defaultLockTTL    = 20 * time.Second
	defaultUpdateTime = 5 * time.Second

	// periodicSync is the time between each call to SyncCluster
	periodicSync = 10 * time.Minute
)

// InitializeEtcd creates a new Etcd client given
// a list of endpoints and optional tls config
func InitializeEtcd(addrs []string, options *Config) (Store, error) {
	s := &Etcd{}

	entries := createEndpoints(addrs, "http")
	s.client = etcd.NewClient(entries)

	// Set options
	if options != nil {
		if options.TLS != nil {
			s.setTLS(options.TLS)
		}
		if options.ConnectionTimeout != 0 {
			s.setTimeout(options.ConnectionTimeout)
		}
		if options.EphemeralTTL != 0 {
			s.setEphemeralTTL(options.EphemeralTTL)
		}
	}

	go func() {
		for {
			s.client.SyncCluster()
			time.Sleep(periodicSync)
		}
	}()
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

// SetHeartbeat sets the heartbeat value to notify we are alive
func (s *Etcd) setEphemeralTTL(time time.Duration) {
	s.ephemeralTTL = time
}

// Create the entire path for a directory that does not exist
func (s *Etcd) createDirectory(path string) error {
	if _, err := s.client.CreateDir(normalize(path), 10); err != nil {
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
func (s *Etcd) Get(key string) (*KVPair, error) {
	result, err := s.client.Get(normalize(key), false, false)
	if err != nil {
		if etcdError, ok := err.(*etcd.EtcdError); ok {
			// Not a Directory or Not a file
			if etcdError.ErrorCode == 102 || etcdError.ErrorCode == 104 {
				return nil, ErrKeyNotFound
			}
		}
		return nil, err
	}
	return &KVPair{key, []byte(result.Node.Value), result.Node.ModifiedIndex}, nil
}

// Put a value at "key"
func (s *Etcd) Put(key string, value []byte, opts *WriteOptions) error {

	// Default TTL = 0 means no expiration
	var ttl uint64
	if opts != nil && opts.Ephemeral {
		ttl = uint64(s.ephemeralTTL.Seconds())
	}

	if _, err := s.client.Set(key, string(value), ttl); err != nil {
		if etcdError, ok := err.(*etcd.EtcdError); ok {
			if etcdError.ErrorCode == 104 { // Not a directory
				// Remove the last element (the actual key) and set the prefix as a dir
				err = s.createDirectory(getDirectory(key))
				if _, err := s.client.Set(key, string(value), ttl); err != nil {
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
	if _, err := s.client.Delete(normalize(key), false); err != nil {
		return err
	}
	return nil
}

// Exists checks if the key exists inside the store
func (s *Etcd) Exists(key string) (bool, error) {
	entry, err := s.Get(key)
	if err != nil {
		if err == ErrKeyNotFound || entry.Value == nil {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Watch changes on a key.
// Returns a channel that will receive changes or an error.
// Upon creating a watch, the current value will be sent to the channel.
// Providing a non-nil stopCh can be used to stop watching.
func (s *Etcd) Watch(key string, stopCh <-chan struct{}) (<-chan *KVPair, error) {
	// Get the current value
	current, err := s.Get(key)
	if err != nil {
		return nil, err
	}

	// Start an etcd watch.
	// Note: etcd will send the current value through the channel.
	etcdWatchCh := make(chan *etcd.Response)
	etcdStopCh := make(chan bool)
	go s.client.Watch(normalize(key), 0, false, etcdWatchCh, etcdStopCh)

	// Adapter goroutine: The goal here is to convert wathever format etcd is
	// using into our interface.
	watchCh := make(chan *KVPair)
	go func() {
		defer close(watchCh)

		// Push the current value through the channel.
		watchCh <- current

		for {
			select {
			case result := <-etcdWatchCh:
				watchCh <- &KVPair{
					key,
					[]byte(result.Node.Value),
					result.Node.ModifiedIndex,
				}
			case <-stopCh:
				etcdStopCh <- true
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
func (s *Etcd) WatchTree(prefix string, stopCh <-chan struct{}) (<-chan []*KVPair, error) {
	// Get the current value
	current, err := s.List(prefix)
	if err != nil {
		return nil, err
	}

	// Start an etcd watch.
	etcdWatchCh := make(chan *etcd.Response)
	etcdStopCh := make(chan bool)
	go s.client.Watch(normalize(prefix), 0, true, etcdWatchCh, etcdStopCh)

	// Adapter goroutine: The goal here is to convert wathever format etcd is
	// using into our interface.
	watchCh := make(chan []*KVPair)
	go func() {
		defer close(watchCh)

		// Push the current value through the channel.
		watchCh <- current

		for {
			select {
			case <-etcdWatchCh:
				// FIXME: We should probably use the value pushed by the channel.
				// However, .Node.Nodes seems to be empty.
				if list, err := s.List(prefix); err == nil {
					watchCh <- list
				}
			case <-stopCh:
				etcdStopCh <- true
				return
			}
		}
	}()
	return watchCh, nil
}

// AtomicPut put a value at "key" if the key has not been
// modified in the meantime, throws an error if this is the case
func (s *Etcd) AtomicPut(key string, value []byte, previous *KVPair, options *WriteOptions) (bool, *KVPair, error) {
	if previous == nil {
		return false, nil, ErrPreviousNotSpecified
	}

	meta, err := s.client.CompareAndSwap(normalize(key), string(value), 0, "", previous.LastIndex)
	if err != nil {
		if etcdError, ok := err.(*etcd.EtcdError); ok {
			if etcdError.ErrorCode == 101 { // Compare failed
				return false, nil, ErrKeyModified
			}
		}
		return false, nil, err
	}
	return true, &KVPair{Key: key, Value: value, LastIndex: meta.Node.ModifiedIndex}, nil
}

// AtomicDelete deletes a value at "key" if the key has not
// been modified in the meantime, throws an error if this is the case
func (s *Etcd) AtomicDelete(key string, previous *KVPair) (bool, error) {
	if previous == nil {
		return false, ErrPreviousNotSpecified
	}

	_, err := s.client.CompareAndDelete(normalize(key), "", previous.LastIndex)
	if err != nil {
		if etcdError, ok := err.(*etcd.EtcdError); ok {
			if etcdError.ErrorCode == 101 { // Compare failed
				return false, ErrKeyModified
			}
		}
		return false, err
	}
	return true, nil
}

// List the content of a given prefix
func (s *Etcd) List(prefix string) ([]*KVPair, error) {
	resp, err := s.client.Get(normalize(prefix), true, true)
	if err != nil {
		return nil, err
	}
	kv := []*KVPair{}
	for _, n := range resp.Node.Nodes {
		key := strings.TrimLeft(n.Key, "/")
		kv = append(kv, &KVPair{key, []byte(n.Value), n.ModifiedIndex})
	}
	return kv, nil
}

// DeleteTree deletes a range of keys based on prefix
func (s *Etcd) DeleteTree(prefix string) error {
	if _, err := s.client.Delete(normalize(prefix), true); err != nil {
		return err
	}
	return nil
}

// NewLock returns a handle to a lock struct which can be used to acquire and
// release the mutex.
func (s *Etcd) NewLock(key string, options *LockOptions) (Locker, error) {
	var value string
	ttl := uint64(time.Duration(defaultLockTTL).Seconds())

	// Apply options
	if options != nil {
		if options.Value != nil {
			value = string(options.Value)
		}
		if options.TTL != 0 {
			ttl = uint64(options.TTL.Seconds())
		}
	}

	// Create lock object
	lock := &etcdLock{
		client: s.client,
		key:    key,
		value:  value,
		ttl:    ttl,
	}

	return lock, nil
}

// Lock attempts to acquire the lock and blocks while doing so.
// Returns a channel that is closed if our lock is lost or an error.
func (l *etcdLock) Lock() (<-chan struct{}, error) {

	key := normalize(l.key)

	// Lock holder channels
	lockHeld := make(chan struct{})
	stopLocking := make(chan struct{})

	var lastIndex uint64

	for {
		resp, err := l.client.Create(key, l.value, l.ttl)
		if err != nil {
			if etcdError, ok := err.(*etcd.EtcdError); ok {
				// Key already exists
				if etcdError.ErrorCode != 105 {
					lastIndex = ^uint64(0)
				}
			}
		} else {
			lastIndex = resp.Node.ModifiedIndex
		}

		_, err = l.client.CompareAndSwap(key, l.value, l.ttl, "", lastIndex)

		if err == nil {
			// Leader section
			l.stopLock = stopLocking
			go l.holdLock(key, lockHeld, stopLocking)
			break
		} else {
			// Seeker section
			chW := make(chan *etcd.Response)
			chWStop := make(chan bool)
			l.waitLock(key, chW, chWStop)

			// Delete or Expire event occured
			// Retry
		}
	}

	return lockHeld, nil
}

// Hold the lock as long as we can
// Updates the key ttl periodically until we receive
// an explicit stop signal from the Unlock method
func (l *etcdLock) holdLock(key string, lockHeld chan struct{}, stopLocking chan struct{}) {
	defer close(lockHeld)

	update := time.NewTicker(defaultUpdateTime)
	defer update.Stop()

	var err error

	for {
		select {
		case <-update.C:
			l.last, err = l.client.Update(key, l.value, l.ttl)
			if err != nil {
				return
			}

		case <-stopLocking:
			return
		}
	}
}

// WaitLock simply waits for the key to be available for creation
func (l *etcdLock) waitLock(key string, eventCh chan *etcd.Response, stopWatchCh chan bool) {
	go l.client.Watch(key, 0, false, eventCh, stopWatchCh)
	for event := range eventCh {
		if event.Action == "delete" || event.Action == "expire" {
			return
		}
	}
}

// Unlock released the lock. It is an error to call this
// if the lock is not currently held.
func (l *etcdLock) Unlock() error {
	if l.stopLock != nil {
		l.stopLock <- struct{}{}
	}
	if l.last != nil {
		_, err := l.client.CompareAndDelete(normalize(l.key), l.value, l.last.Node.ModifiedIndex)
		if err != nil {
			return err
		}
	}
	return nil
}

// Close closes the client connection
func (s *Etcd) Close() {
	return
}

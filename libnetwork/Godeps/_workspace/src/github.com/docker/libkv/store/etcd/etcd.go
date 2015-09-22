package etcd

import (
	"crypto/tls"
	"net"
	"net/http"
	"strings"
	"time"

	etcd "github.com/coreos/go-etcd/etcd"
	"github.com/docker/libkv"
	"github.com/docker/libkv/store"
)

// Etcd is the receiver type for the
// Store interface
type Etcd struct {
	client *etcd.Client
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
	periodicSync      = 10 * time.Minute
	defaultLockTTL    = 20 * time.Second
	defaultUpdateTime = 5 * time.Second
)

// Register registers etcd to libkv
func Register() {
	libkv.AddStore(store.ETCD, New)
}

// New creates a new Etcd client given a list
// of endpoints and an optional tls config
func New(addrs []string, options *store.Config) (store.Store, error) {
	s := &Etcd{}

	var (
		entries []string
		err     error
	)

	// Create the etcd client
	if options != nil && options.ClientTLS != nil {
		entries = store.CreateEndpoints(addrs, "https")
		s.client, err = etcd.NewTLSClient(entries, options.ClientTLS.CertFile, options.ClientTLS.KeyFile, options.ClientTLS.CACertFile)
		if err != nil {
			return nil, err
		}
	} else {
		entries = store.CreateEndpoints(addrs, "http")
		s.client = etcd.NewClient(entries)
	}

	// Set options
	if options != nil {
		// Plain TLS config overrides ClientTLS if specified
		if options.TLS != nil {
			s.setTLS(options.TLS, addrs)
		}
		if options.ConnectionTimeout != 0 {
			s.setTimeout(options.ConnectionTimeout)
		}
	}

	// Periodic SyncCluster
	go func() {
		for {
			s.client.SyncCluster()
			time.Sleep(periodicSync)
		}
	}()

	return s, nil
}

// SetTLS sets the tls configuration given a tls.Config scheme
func (s *Etcd) setTLS(tls *tls.Config, addrs []string) {
	entries := store.CreateEndpoints(addrs, "https")
	s.client.SetCluster(entries)

	// Set transport
	t := http.Transport{
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     tls,
	}
	s.client.SetTransport(&t)
}

// setTimeout sets the timeout used for connecting to the store
func (s *Etcd) setTimeout(time time.Duration) {
	s.client.SetDialTimeout(time)
}

// createDirectory creates the entire path for a directory
// that does not exist
func (s *Etcd) createDirectory(path string) error {
	if _, err := s.client.CreateDir(store.Normalize(path), 10); err != nil {
		if etcdError, ok := err.(*etcd.EtcdError); ok {
			// Skip key already exists
			if etcdError.ErrorCode != 105 {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

// Get the value at "key", returns the last modified index
// to use in conjunction to Atomic calls
func (s *Etcd) Get(key string) (pair *store.KVPair, err error) {
	result, err := s.client.Get(store.Normalize(key), false, false)
	if err != nil {
		if isKeyNotFoundError(err) {
			return nil, store.ErrKeyNotFound
		}
		return nil, err
	}

	pair = &store.KVPair{
		Key:       key,
		Value:     []byte(result.Node.Value),
		LastIndex: result.Node.ModifiedIndex,
	}

	return pair, nil
}

// Put a value at "key"
func (s *Etcd) Put(key string, value []byte, opts *store.WriteOptions) error {

	// Default TTL = 0 means no expiration
	var ttl uint64
	if opts != nil && opts.TTL > 0 {
		ttl = uint64(opts.TTL.Seconds())
	}

	if _, err := s.client.Set(key, string(value), ttl); err != nil {
		if etcdError, ok := err.(*etcd.EtcdError); ok {

			// Not a directory
			if etcdError.ErrorCode == 104 {
				// Remove the last element (the actual key)
				// and create the full directory path
				err = s.createDirectory(store.GetDirectory(key))
				if err != nil {
					return err
				}

				// Now that the directory is created, set the key
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
	_, err := s.client.Delete(store.Normalize(key), false)
	if isKeyNotFoundError(err) {
		return store.ErrKeyNotFound
	}
	return err
}

// Exists checks if the key exists inside the store
func (s *Etcd) Exists(key string) (bool, error) {
	_, err := s.Get(key)
	if err != nil {
		if err == store.ErrKeyNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Watch for changes on a "key"
// It returns a channel that will receive changes or pass
// on errors. Upon creation, the current value will first
// be sent to the channel. Providing a non-nil stopCh can
// be used to stop watching.
func (s *Etcd) Watch(key string, stopCh <-chan struct{}) (<-chan *store.KVPair, error) {
	// Start an etcd watch.
	// Note: etcd will send the current value through the channel.
	etcdWatchCh := make(chan *etcd.Response)
	etcdStopCh := make(chan bool)
	go s.client.Watch(store.Normalize(key), 0, false, etcdWatchCh, etcdStopCh)

	// Adapter goroutine: The goal here is to convert whatever
	// format etcd is using into our interface.
	watchCh := make(chan *store.KVPair)
	go func() {
		defer close(watchCh)

		// Get the current value
		current, err := s.Get(key)
		if err != nil {
			return
		}

		// Push the current value through the channel.
		watchCh <- current

		for {
			select {
			case result := <-etcdWatchCh:
				if result == nil || result.Node == nil {
					// Something went wrong, exit
					// No need to stop the chan as the watch already ended
					return
				}
				watchCh <- &store.KVPair{
					Key:       key,
					Value:     []byte(result.Node.Value),
					LastIndex: result.Node.ModifiedIndex,
				}
			case <-stopCh:
				etcdStopCh <- true
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
func (s *Etcd) WatchTree(directory string, stopCh <-chan struct{}) (<-chan []*store.KVPair, error) {
	// Start the watch
	etcdWatchCh := make(chan *etcd.Response)
	etcdStopCh := make(chan bool)
	go s.client.Watch(store.Normalize(directory), 0, true, etcdWatchCh, etcdStopCh)

	// Adapter goroutine: The goal here is to convert whatever
	// format etcd is using into our interface.
	watchCh := make(chan []*store.KVPair)
	go func() {
		defer close(watchCh)

		// Get child values
		current, err := s.List(directory)
		if err != nil {
			return
		}

		// Push the current value through the channel.
		watchCh <- current

		for {
			select {
			case event := <-etcdWatchCh:
				if event == nil {
					// Something went wrong, exit
					// No need to stop the chan as the watch already ended
					return
				}
				// FIXME: We should probably use the value pushed by the channel.
				// However, Node.Nodes seems to be empty.
				if list, err := s.List(directory); err == nil {
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
func (s *Etcd) AtomicPut(key string, value []byte, previous *store.KVPair, options *store.WriteOptions) (bool, *store.KVPair, error) {

	var meta *etcd.Response
	var err error
	if previous != nil {
		meta, err = s.client.CompareAndSwap(store.Normalize(key), string(value), 0, "", previous.LastIndex)
	} else {
		// Interpret previous == nil as Atomic Create
		meta, err = s.client.Create(store.Normalize(key), string(value), 0)
		if etcdError, ok := err.(*etcd.EtcdError); ok {

			// Directory doesn't exist.
			if etcdError.ErrorCode == 104 {
				// Remove the last element (the actual key)
				// and create the full directory path
				err = s.createDirectory(store.GetDirectory(key))
				if err != nil {
					return false, nil, err
				}

				// Now that the directory is created, create the key
				if _, err := s.client.Create(key, string(value), 0); err != nil {
					return false, nil, err
				}
			}
		}
	}
	if err != nil {
		if etcdError, ok := err.(*etcd.EtcdError); ok {
			// Compare Failed
			if etcdError.ErrorCode == 101 {
				return false, nil, store.ErrKeyModified
			}
		}
		return false, nil, err
	}

	updated := &store.KVPair{
		Key:       key,
		Value:     value,
		LastIndex: meta.Node.ModifiedIndex,
	}

	return true, updated, nil
}

// AtomicDelete deletes a value at "key" if the key
// has not been modified in the meantime, throws an
// error if this is the case
func (s *Etcd) AtomicDelete(key string, previous *store.KVPair) (bool, error) {
	if previous == nil {
		return false, store.ErrPreviousNotSpecified
	}

	_, err := s.client.CompareAndDelete(store.Normalize(key), "", previous.LastIndex)
	if err != nil {
		if etcdError, ok := err.(*etcd.EtcdError); ok {
			// Compare failed
			if etcdError.ErrorCode == 101 {
				return false, store.ErrKeyModified
			}
		}
		return false, err
	}

	return true, nil
}

// List child nodes of a given directory
func (s *Etcd) List(directory string) ([]*store.KVPair, error) {
	resp, err := s.client.Get(store.Normalize(directory), true, true)
	if err != nil {
		if isKeyNotFoundError(err) {
			return nil, store.ErrKeyNotFound
		}
		return nil, err
	}
	kv := []*store.KVPair{}
	for _, n := range resp.Node.Nodes {
		key := strings.TrimLeft(n.Key, "/")
		kv = append(kv, &store.KVPair{
			Key:       key,
			Value:     []byte(n.Value),
			LastIndex: n.ModifiedIndex,
		})
	}
	return kv, nil
}

// DeleteTree deletes a range of keys under a given directory
func (s *Etcd) DeleteTree(directory string) error {
	_, err := s.client.Delete(store.Normalize(directory), true)
	if isKeyNotFoundError(err) {
		return store.ErrKeyNotFound
	}
	return err
}

// NewLock returns a handle to a lock struct which can
// be used to provide mutual exclusion on a key
func (s *Etcd) NewLock(key string, options *store.LockOptions) (lock store.Locker, err error) {
	var value string
	ttl := uint64(time.Duration(defaultLockTTL).Seconds())

	// Apply options on Lock
	if options != nil {
		if options.Value != nil {
			value = string(options.Value)
		}
		if options.TTL != 0 {
			ttl = uint64(options.TTL.Seconds())
		}
	}

	// Create lock object
	lock = &etcdLock{
		client: s.client,
		key:    key,
		value:  value,
		ttl:    ttl,
	}

	return lock, nil
}

// Lock attempts to acquire the lock and blocks while
// doing so. It returns a channel that is closed if our
// lock is lost or if an error occurs
func (l *etcdLock) Lock() (<-chan struct{}, error) {

	key := store.Normalize(l.key)

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

		l.last, err = l.client.CompareAndSwap(key, l.value, l.ttl, "", lastIndex)

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
			l.last, err = l.client.CompareAndSwap(key, l.value, l.ttl, "", l.last.Node.ModifiedIndex)
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

// Unlock the "key". Calling unlock while
// not holding the lock will throw an error
func (l *etcdLock) Unlock() error {
	if l.stopLock != nil {
		l.stopLock <- struct{}{}
	}
	if l.last != nil {
		_, err := l.client.CompareAndDelete(store.Normalize(l.key), l.value, l.last.Node.ModifiedIndex)
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

func isKeyNotFoundError(err error) bool {
	if err != nil {
		if etcdError, ok := err.(*etcd.EtcdError); ok {
			// Not a Directory or Not a file
			if etcdError.ErrorCode == 100 || etcdError.ErrorCode == 102 || etcdError.ErrorCode == 104 {
				return true
			}
		}
	}
	return false
}

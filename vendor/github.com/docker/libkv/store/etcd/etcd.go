package etcd

import (
	"context"
	"crypto/tls"
	"errors"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	etcd "github.com/coreos/etcd/client"
	"github.com/docker/libkv"
	"github.com/docker/libkv/store"
)

var (
	// ErrAbortTryLock is thrown when a user stops trying to seek the lock
	// by sending a signal to the stop chan, this is used to verify if the
	// operation succeeded
	ErrAbortTryLock = errors.New("lock operation aborted")
)

// Etcd is the receiver type for the
// Store interface
type Etcd struct {
	client etcd.KeysAPI
}

type etcdLock struct {
	client etcd.KeysAPI
	key    string
	value  string
	ttl    time.Duration

	// Closed when the caller wants to stop renewing the lock. I'm not sure
	// why this is even used - you could just call the Unlock() method.
	stopRenew chan struct{}
	// When the lock is held, this is the last modified index of the key.
	// Used for conditional updates when extending the lock TTL and when
	// conditionall deleteing when Unlock() is called.
	lastIndex uint64
	// When the lock is held, this function will cancel the locked context.
	// This is called both by the Unlock() method in order to stop the
	// background holding goroutine and in a deferred call in that background
	// holding goroutine in case the lock is lost due to an error or the
	// stopRenew channel is closed. Calling this function also closes the chan
	// returned by the Lock() method.
	cancel context.CancelFunc
	// Used to sync the Unlock() call with the background holding goroutine.
	// This channel is closed when that background goroutine exits, signalling
	// that it is okay to conditionally delete the key.
	doneHolding chan struct{}
}

const (
	periodicSync      = 5 * time.Minute
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

	entries = store.CreateEndpoints(addrs, "http")
	cfg := &etcd.Config{
		Endpoints:               entries,
		Transport:               etcd.DefaultTransport,
		HeaderTimeoutPerRequest: 3 * time.Second,
	}

	// Set options
	if options != nil {
		if options.TLS != nil {
			setTLS(cfg, options.TLS, addrs)
		}
		if options.ConnectionTimeout != 0 {
			setTimeout(cfg, options.ConnectionTimeout)
		}
		if options.Username != "" {
			setCredentials(cfg, options.Username, options.Password)
		}
	}

	c, err := etcd.New(*cfg)
	if err != nil {
		log.Fatal(err)
	}

	s.client = etcd.NewKeysAPI(c)

	// Periodic Cluster Sync
	go func() {
		for {
			if err := c.AutoSync(context.Background(), periodicSync); err != nil {
				return
			}
		}
	}()

	return s, nil
}

// SetTLS sets the tls configuration given a tls.Config scheme
func setTLS(cfg *etcd.Config, tls *tls.Config, addrs []string) {
	entries := store.CreateEndpoints(addrs, "https")
	cfg.Endpoints = entries

	// Set transport
	t := http.Transport{
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     tls,
	}

	cfg.Transport = &t
}

// setTimeout sets the timeout used for connecting to the store
func setTimeout(cfg *etcd.Config, time time.Duration) {
	cfg.HeaderTimeoutPerRequest = time
}

// setCredentials sets the username/password credentials for connecting to Etcd
func setCredentials(cfg *etcd.Config, username, password string) {
	cfg.Username = username
	cfg.Password = password
}

// Normalize the key for usage in Etcd
func (s *Etcd) normalize(key string) string {
	key = store.Normalize(key)
	return strings.TrimPrefix(key, "/")
}

// keyNotFound checks on the error returned by the KeysAPI
// to verify if the key exists in the store or not
func keyNotFound(err error) bool {
	if err != nil {
		if etcdError, ok := err.(etcd.Error); ok {
			if etcdError.Code == etcd.ErrorCodeKeyNotFound ||
				etcdError.Code == etcd.ErrorCodeNotFile ||
				etcdError.Code == etcd.ErrorCodeNotDir {
				return true
			}
		}
	}
	return false
}

// Get the value at "key", returns the last modified
// index to use in conjunction to Atomic calls
func (s *Etcd) Get(key string) (pair *store.KVPair, err error) {
	getOpts := &etcd.GetOptions{
		Quorum: true,
	}

	result, err := s.client.Get(context.Background(), s.normalize(key), getOpts)
	if err != nil {
		if keyNotFound(err) {
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
	setOpts := &etcd.SetOptions{}

	// Set options
	if opts != nil {
		setOpts.Dir = opts.IsDir
		setOpts.TTL = opts.TTL
	}

	_, err := s.client.Set(context.Background(), s.normalize(key), string(value), setOpts)
	return err
}

// Delete a value at "key"
func (s *Etcd) Delete(key string) error {
	opts := &etcd.DeleteOptions{
		Recursive: false,
	}

	_, err := s.client.Delete(context.Background(), s.normalize(key), opts)
	if keyNotFound(err) {
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
	opts := &etcd.WatcherOptions{Recursive: false}
	watcher := s.client.Watcher(s.normalize(key), opts)

	// watchCh is sending back events to the caller
	watchCh := make(chan *store.KVPair)

	go func() {
		defer close(watchCh)

		// Get the current value
		pair, err := s.Get(key)
		if err != nil {
			return
		}

		// Push the current value through the channel.
		watchCh <- pair

		for {
			// Check if the watch was stopped by the caller
			select {
			case <-stopCh:
				return
			default:
			}

			result, err := watcher.Next(context.Background())

			if err != nil {
				return
			}

			watchCh <- &store.KVPair{
				Key:       key,
				Value:     []byte(result.Node.Value),
				LastIndex: result.Node.ModifiedIndex,
			}
		}
	}()

	return watchCh, nil
}

// WatchTree watches for changes on a "directory"
// It returns a channel that will receive changes or pass
// on errors. Upon creating a watch, the current childs values
// will be sent to the channel. Providing a non-nil stopCh can
// be used to stop watching.
func (s *Etcd) WatchTree(directory string, stopCh <-chan struct{}) (<-chan []*store.KVPair, error) {
	watchOpts := &etcd.WatcherOptions{Recursive: true}
	watcher := s.client.Watcher(s.normalize(directory), watchOpts)

	// watchCh is sending back events to the caller
	watchCh := make(chan []*store.KVPair)

	go func() {
		defer close(watchCh)

		// Get child values
		list, err := s.List(directory)
		if err != nil {
			return
		}

		// Push the current value through the channel.
		watchCh <- list

		for {
			// Check if the watch was stopped by the caller
			select {
			case <-stopCh:
				return
			default:
			}

			_, err := watcher.Next(context.Background())

			if err != nil {
				return
			}

			list, err = s.List(directory)
			if err != nil {
				return
			}

			watchCh <- list
		}
	}()

	return watchCh, nil
}

// AtomicPut puts a value at "key" if the key has not been
// modified in the meantime, throws an error if this is the case
func (s *Etcd) AtomicPut(key string, value []byte, previous *store.KVPair, opts *store.WriteOptions) (bool, *store.KVPair, error) {
	var (
		meta *etcd.Response
		err  error
	)

	setOpts := &etcd.SetOptions{}

	if previous != nil {
		setOpts.PrevExist = etcd.PrevExist
		setOpts.PrevIndex = previous.LastIndex
		if previous.Value != nil {
			setOpts.PrevValue = string(previous.Value)
		}
	} else {
		setOpts.PrevExist = etcd.PrevNoExist
	}

	if opts != nil {
		if opts.TTL > 0 {
			setOpts.TTL = opts.TTL
		}
	}

	meta, err = s.client.Set(context.Background(), s.normalize(key), string(value), setOpts)
	if err != nil {
		if etcdError, ok := err.(etcd.Error); ok {
			// Compare failed
			if etcdError.Code == etcd.ErrorCodeTestFailed {
				return false, nil, store.ErrKeyModified
			}
			// Node exists error (when PrevNoExist)
			if etcdError.Code == etcd.ErrorCodeNodeExist {
				return false, nil, store.ErrKeyExists
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

	delOpts := &etcd.DeleteOptions{}

	if previous != nil {
		delOpts.PrevIndex = previous.LastIndex
		if previous.Value != nil {
			delOpts.PrevValue = string(previous.Value)
		}
	}

	_, err := s.client.Delete(context.Background(), s.normalize(key), delOpts)
	if err != nil {
		if etcdError, ok := err.(etcd.Error); ok {
			// Key Not Found
			if etcdError.Code == etcd.ErrorCodeKeyNotFound {
				return false, store.ErrKeyNotFound
			}
			// Compare failed
			if etcdError.Code == etcd.ErrorCodeTestFailed {
				return false, store.ErrKeyModified
			}
		}
		return false, err
	}

	return true, nil
}

// List child nodes of a given directory
func (s *Etcd) List(directory string) ([]*store.KVPair, error) {
	getOpts := &etcd.GetOptions{
		Quorum:    true,
		Recursive: true,
		Sort:      true,
	}

	resp, err := s.client.Get(context.Background(), s.normalize(directory), getOpts)
	if err != nil {
		if keyNotFound(err) {
			return nil, store.ErrKeyNotFound
		}
		return nil, err
	}

	kv := []*store.KVPair{}
	for _, n := range resp.Node.Nodes {
		kv = append(kv, &store.KVPair{
			Key:       n.Key,
			Value:     []byte(n.Value),
			LastIndex: n.ModifiedIndex,
		})
	}
	return kv, nil
}

// DeleteTree deletes a range of keys under a given directory
func (s *Etcd) DeleteTree(directory string) error {
	delOpts := &etcd.DeleteOptions{
		Recursive: true,
	}

	_, err := s.client.Delete(context.Background(), s.normalize(directory), delOpts)
	if keyNotFound(err) {
		return store.ErrKeyNotFound
	}
	return err
}

// NewLock returns a handle to a lock struct which can
// be used to provide mutual exclusion on a key
func (s *Etcd) NewLock(key string, options *store.LockOptions) (lock store.Locker, err error) {
	var value string
	ttl := defaultLockTTL
	renewCh := make(chan struct{})

	// Apply options on Lock
	if options != nil {
		if options.Value != nil {
			value = string(options.Value)
		}
		if options.TTL != 0 {
			ttl = options.TTL
		}
		if options.RenewLock != nil {
			renewCh = options.RenewLock
		}
	}

	// Create lock object
	lock = &etcdLock{
		client:    s.client,
		stopRenew: renewCh,
		key:       s.normalize(key),
		value:     value,
		ttl:       ttl,
	}

	return lock, nil
}

// Lock attempts to acquire the lock and blocks while
// doing so. It returns a channel that is closed if our
// lock is lost or if an error occurs
func (l *etcdLock) Lock(stopChan chan struct{}) (<-chan struct{}, error) {
	// Conditional Set - only if the key does not exist.
	setOpts := &etcd.SetOptions{
		TTL:       l.ttl,
		PrevExist: etcd.PrevNoExist,
	}

	for {
		resp, err := l.client.Set(context.Background(), l.key, l.value, setOpts)
		if err == nil {
			// Acquired the lock!
			l.lastIndex = resp.Node.ModifiedIndex
			lockedCtx, cancel := context.WithCancel(context.Background())
			l.cancel = cancel
			l.doneHolding = make(chan struct{})

			go l.holdLock(lockedCtx)

			return lockedCtx.Done(), nil
		}

		etcdErr, ok := err.(etcd.Error)
		if !ok || etcdErr.Code != etcd.ErrorCodeNodeExist {
			return nil, err // Unexpected error.
		}

		// Need to wait for the lock key to expire or be deleted.
		if err := l.waitLock(stopChan, etcdErr.Index); err != nil {
			return nil, err
		}

		// Delete or Expire event occurred.
		// Retry
	}
}

// Hold the lock as long as we can.
// Updates the key ttl periodically until we receive
// an explicit stop signal from the Unlock method OR
// the stopRenew channel is closed.
func (l *etcdLock) holdLock(ctx context.Context) {
	defer close(l.doneHolding)
	defer l.cancel()

	update := time.NewTicker(l.ttl / 3)
	defer update.Stop()

	setOpts := &etcd.SetOptions{TTL: l.ttl}

	for {
		select {
		case <-update.C:
			setOpts.PrevIndex = l.lastIndex
			resp, err := l.client.Set(ctx, l.key, l.value, setOpts)
			if err != nil {
				return
			}
			l.lastIndex = resp.Node.ModifiedIndex
		case <-l.stopRenew:
			return
		case <-ctx.Done():
			return
		}
	}
}

// WaitLock simply waits for the key to be available for creation.
func (l *etcdLock) waitLock(stopWait <-chan struct{}, afterIndex uint64) error {
	waitCtx, waitCancel := context.WithCancel(context.Background())
	defer waitCancel()
	go func() {
		select {
		case <-stopWait:
			// If the caller closes the stopWait, cancel the wait context.
			waitCancel()
		case <-waitCtx.Done():
			// No longer waiting.
		}
	}()

	watcher := l.client.Watcher(l.key, &etcd.WatcherOptions{AfterIndex: afterIndex})
	for {
		event, err := watcher.Next(waitCtx)
		if err != nil {
			if err == context.Canceled {
				return ErrAbortTryLock
			}
			return err
		}
		switch event.Action {
		case "delete", "compareAndDelete", "expire":
			return nil // The key has been deleted or expired.
		}
	}
}

// Unlock the "key". Calling unlock while
// not holding the lock will throw an error
func (l *etcdLock) Unlock() error {
	l.cancel()      // Will signal the holdLock goroutine to exit.
	<-l.doneHolding // Wait for the holdLock goroutine to exit.

	var err error
	if l.lastIndex != 0 {
		delOpts := &etcd.DeleteOptions{
			PrevIndex: l.lastIndex,
		}
		_, err = l.client.Delete(context.Background(), l.key, delOpts)
	}
	return err
}

// Close closes the client connection
func (s *Etcd) Close() {
	return
}

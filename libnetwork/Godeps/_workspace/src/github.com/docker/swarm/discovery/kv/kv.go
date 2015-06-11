package kv

import (
	"fmt"
	"path"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/swarm/discovery"
	"github.com/docker/swarm/pkg/store"
)

const (
	discoveryPath = "docker/swarm/nodes"
)

// Discovery is exported
type Discovery struct {
	backend   store.Backend
	store     store.Store
	heartbeat time.Duration
	ttl       time.Duration
	path      string
}

func init() {
	Init()
}

// Init is exported
func Init() {
	discovery.Register("zk", &Discovery{backend: store.ZK})
	discovery.Register("consul", &Discovery{backend: store.CONSUL})
	discovery.Register("etcd", &Discovery{backend: store.ETCD})
}

// Initialize is exported
func (s *Discovery) Initialize(uris string, heartbeat time.Duration, ttl time.Duration) error {
	var (
		parts  = strings.SplitN(uris, "/", 2)
		addrs  = strings.Split(parts[0], ",")
		prefix = ""
		err    error
	)

	// A custom prefix to the path can be optionally used.
	if len(parts) == 2 {
		prefix = parts[1]
	}

	s.heartbeat = heartbeat
	s.ttl = ttl
	s.path = path.Join(prefix, discoveryPath)

	// Creates a new store, will ignore options given
	// if not supported by the chosen store
	s.store, err = store.NewStore(
		s.backend,
		addrs,
		&store.Config{
			EphemeralTTL: s.ttl,
		},
	)

	return err
}

// Watch the store until either there's a store error or we receive a stop request.
// Returns false if we shouldn't attempt watching the store anymore (stop request received).
func (s *Discovery) watchOnce(stopCh <-chan struct{}, watchCh <-chan []*store.KVPair, discoveryCh chan discovery.Entries, errCh chan error) bool {
	for {
		select {
		case pairs := <-watchCh:
			if pairs == nil {
				return true
			}

			log.WithField("discovery", s.backend).Debugf("Watch triggered with %d nodes", len(pairs))

			// Convert `KVPair` into `discovery.Entry`.
			addrs := make([]string, len(pairs))
			for _, pair := range pairs {
				addrs = append(addrs, string(pair.Value))
			}

			entries, err := discovery.CreateEntries(addrs)
			if err != nil {
				errCh <- err
			} else {
				discoveryCh <- entries
			}
		case <-stopCh:
			// We were requested to stop watching.
			return false
		}
	}
}

// Watch is exported
func (s *Discovery) Watch(stopCh <-chan struct{}) (<-chan discovery.Entries, <-chan error) {
	ch := make(chan discovery.Entries)
	errCh := make(chan error)

	go func() {
		defer close(ch)
		defer close(errCh)

		// Forever: Create a store watch, watch until we get an error and then try again.
		// Will only stop if we receive a stopCh request.
		for {
			// Set up a watch.
			watchCh, err := s.store.WatchTree(s.path, stopCh)
			if err != nil {
				errCh <- err
			} else {
				if !s.watchOnce(stopCh, watchCh, ch, errCh) {
					return
				}
			}

			// If we get here it means the store watch channel was closed. This
			// is unexpected so let's retry later.
			errCh <- fmt.Errorf("Unexpected watch error")
			time.Sleep(s.heartbeat)
		}
	}()
	return ch, errCh
}

// Register is exported
func (s *Discovery) Register(addr string) error {
	opts := &store.WriteOptions{Ephemeral: true, Heartbeat: s.heartbeat}
	return s.store.Put(path.Join(s.path, addr), []byte(addr), opts)
}

// Store returns the underlying store used by KV discovery.
func (s *Discovery) Store() store.Store {
	return s.store
}

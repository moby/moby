package libnetwork

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/moby/moby/v2/daemon/libnetwork/config"
	"github.com/moby/moby/v2/daemon/libnetwork/datastore"
	store "github.com/moby/moby/v2/daemon/libnetwork/kvstore"
	"github.com/moby/moby/v2/daemon/libnetwork/kvstore/boltdb"
	"github.com/moby/moby/v2/daemon/libnetwork/kvstore/memstore"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestBoltdbBackend(t *testing.T) {
	tmpPath := filepath.Join(t.TempDir(), "boltdb.db")
	testLocalBackend(t, tmpPath)
}

func TestNoPersist(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "local-kv.db")
	kv, err := boltdb.New(dbPath)
	if err != nil {
		t.Fatalf("Error opening datastore: %v", err)
	}
	testController, err := New(t.Context(), config.WithKVStore(kv))
	if err != nil {
		t.Fatalf("Error creating new controller: %v", err)
	}
	nw, err := testController.NewNetwork(t.Context(), "host", "host", "", NetworkOptionPersist(false))
	if err != nil {
		t.Fatalf(`Error creating default "host" network: %v`, err)
	}
	ep, err := nw.CreateEndpoint(t.Context(), "newendpoint", []EndpointOption{}...)
	if err != nil {
		t.Fatalf("Error creating endpoint: %v", err)
	}
	kv.Close()

	// Create a new controller using the same database-file. The network
	// should not have persisted.
	kv, err = boltdb.New(dbPath)
	if err != nil {
		t.Fatalf("Error opening datastore: %v", err)
	}
	defer kv.Close()
	testController, err = New(t.Context(), config.WithKVStore(kv))
	if err != nil {
		t.Fatalf("Error creating new controller: %v", err)
	}

	nwKVObject := &Network{id: nw.ID()}
	err = testController.store.GetObject(nwKVObject)
	if !errors.Is(err, store.ErrKeyNotFound) {
		t.Errorf("Expected %q error when retrieving network from store, got: %q", store.ErrKeyNotFound, err)
	}
	if nwKVObject.Exists() {
		t.Errorf("Network with persist=false should not be stored in KV Store")
	}

	epKVObject := &Endpoint{network: nw, id: ep.ID()}
	err = testController.store.GetObject(epKVObject)
	if !errors.Is(err, store.ErrKeyNotFound) {
		t.Errorf("Expected %v error when retrieving endpoint from store, got: %v", store.ErrKeyNotFound, err)
	}
	if epKVObject.Exists() {
		t.Errorf("Endpoint in Network with persist=false should not be stored in KV Store")
	}
}

// lockCheckingStore checks, on each change to a network or endpoint in the
// store, that the Controller holds the lock of the matching in-memory cache.
type lockCheckingStore struct {
	store.Store
	c          *Controller
	checked    []string
	violations []string
}

func (s *lockCheckingStore) check(op, key string) {
	if s.c == nil {
		return // The Controller is being created.
	}
	var mu *sync.Mutex
	switch {
	case strings.HasPrefix(key, datastore.Key(datastore.NetworkKeyPrefix)):
		mu = &s.c.networksMu
	case strings.HasPrefix(key, datastore.Key(datastore.EndpointKeyPrefix)):
		mu = &s.c.endpointsMu
	default:
		return
	}
	s.checked = append(s.checked, op+" "+key)
	if mu.TryLock() {
		mu.Unlock()
		s.violations = append(s.violations, op+" "+key)
	}
}

func (s *lockCheckingStore) Put(key string, value []byte) error {
	s.check("Put", key)
	return s.Store.Put(key, value)
}

func (s *lockCheckingStore) AtomicPut(key string, value []byte, previous *store.KVPair) (*store.KVPair, error) {
	s.check("AtomicPut", key)
	return s.Store.AtomicPut(key, value, previous)
}

func (s *lockCheckingStore) AtomicDelete(key string, previous *store.KVPair) error {
	s.check("AtomicDelete", key)
	return s.Store.AtomicDelete(key, previous)
}

func (s *lockCheckingStore) Delete(key string) error {
	s.check("Delete", key)
	return s.Store.Delete(key)
}

// Each change to a network or endpoint in the store must happen together with
// the change to the in-memory cache, under the cache's lock. Otherwise a store
// racing a delete can leave the cache disagreeing with the store.
func TestStoreChangesHoldCacheLock(t *testing.T) {
	kv := &lockCheckingStore{Store: memstore.New()}
	c, err := New(t.Context(), config.WithKVStore(kv))
	assert.NilError(t, err)
	kv.c = c

	nw, err := c.NewNetwork(t.Context(), "host", "host", "")
	assert.NilError(t, err)
	ep, err := nw.CreateEndpoint(t.Context(), "testEndpoint")
	assert.NilError(t, err)
	assert.NilError(t, ep.Delete(t.Context(), false))
	// The host driver refuses to delete the network, after Network.Delete
	// has marked it for deletion in the store.
	assert.Check(t, is.ErrorContains(nw.Delete(), "cannot be deleted"))
	assert.NilError(t, c.deleteStoredNetwork(nw))

	for _, prefix := range []string{datastore.NetworkKeyPrefix, datastore.EndpointKeyPrefix} {
		assert.Check(t, slices.ContainsFunc(kv.checked, func(op string) bool {
			return strings.Contains(op, datastore.Key(prefix))
		}), "no change to a key under %q was checked: %v", prefix, kv.checked)
	}
	assert.Check(t, is.Len(kv.violations, 0), "store changes without the cache lock: %v", kv.violations)
}

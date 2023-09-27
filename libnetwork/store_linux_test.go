package libnetwork

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/docker/libnetwork/config"
	"github.com/docker/docker/libnetwork/datastore"
	store "github.com/docker/docker/libnetwork/internal/kvstore"
	"github.com/docker/docker/testutil"
)

func TestBoltdbBackend(t *testing.T) {
	testutil.SkipWhenUnprivileged(t)

	defer os.Remove(datastore.DefaultScope("").Client.Address)
	testLocalBackend(t, "", "", nil)
	tmpPath := filepath.Join(t.TempDir(), "boltdb.db")
	testLocalBackend(t, "boltdb", tmpPath, &store.Config{
		Bucket: "testBackend",
	})
}

func TestNoPersist(t *testing.T) {
	testutil.SkipWhenUnprivileged(t)

	dbFile := filepath.Join(t.TempDir(), "bolt.db")
	configOption := func(c *config.Config) {
		c.Scope.Client.Provider = "boltdb"
		c.Scope.Client.Address = dbFile
		c.Scope.Client.Config = &store.Config{Bucket: "testBackend"}
	}
	testController, err := New(configOption)
	if err != nil {
		t.Fatalf("Error creating new controller: %v", err)
	}
	defer testController.Stop()
	nw, err := testController.NewNetwork("host", "host", "", NetworkOptionPersist(false))
	if err != nil {
		t.Fatalf(`Error creating default "host" network: %v`, err)
	}
	ep, err := nw.CreateEndpoint("newendpoint", []EndpointOption{}...)
	if err != nil {
		t.Fatalf("Error creating endpoint: %v", err)
	}
	testController.Stop()

	// Create a new controller using the same database-file. The network
	// should not have persisted.
	testController, err = New(configOption)
	if err != nil {
		t.Fatalf("Error creating new controller: %v", err)
	}
	defer testController.Stop()

	// FIXME(thaJeztah): GetObject uses the given key for lookups if no cache-store is present, but the KvObject's Key() to look up in cache....
	nwKVObject := &Network{id: nw.ID()}
	err = testController.getStore().GetObject(datastore.Key(datastore.NetworkKeyPrefix, nw.ID()), nwKVObject)
	if !errors.Is(err, store.ErrKeyNotFound) {
		t.Errorf("Expected %q error when retrieving network from store, got: %q", store.ErrKeyNotFound, err)
	}
	if nwKVObject.Exists() {
		t.Errorf("Network with persist=false should not be stored in KV Store")
	}

	epKVObject := &Endpoint{network: nw, id: ep.ID()}
	err = testController.getStore().GetObject(datastore.Key(datastore.EndpointKeyPrefix, nw.ID(), ep.ID()), epKVObject)
	if !errors.Is(err, store.ErrKeyNotFound) {
		t.Errorf("Expected %v error when retrieving endpoint from store, got: %v", store.ErrKeyNotFound, err)
	}
	if epKVObject.Exists() {
		t.Errorf("Endpoint in Network with persist=false should not be stored in KV Store")
	}
}

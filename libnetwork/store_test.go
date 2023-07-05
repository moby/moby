package libnetwork

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/docker/libnetwork/config"
	"github.com/docker/docker/libnetwork/datastore"
	store "github.com/docker/docker/libnetwork/internal/kvstore"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/options"
)

func testLocalBackend(t *testing.T, provider, url string, storeConfig *store.Config) {
	cfgOptions := []config.Option{func(c *config.Config) {
		c.Scope.Client.Provider = provider
		c.Scope.Client.Address = url
		c.Scope.Client.Config = storeConfig
	}}

	cfgOptions = append(cfgOptions, config.OptionDriverConfig("host", map[string]interface{}{
		netlabel.GenericData: options.Generic{},
	}))

	testController, err := New(cfgOptions...)
	if err != nil {
		t.Fatalf("Error new controller: %v", err)
	}
	defer testController.Stop()
	nw, err := testController.NewNetwork("host", "host", "")
	if err != nil {
		t.Fatalf(`Error creating default "host" network: %v`, err)
	}
	ep, err := nw.CreateEndpoint("newendpoint", []EndpointOption{}...)
	if err != nil {
		t.Fatalf("Error creating endpoint: %v", err)
	}
	kvStore := testController.getStore().KVStore()
	if exists, err := kvStore.Exists(datastore.Key(datastore.NetworkKeyPrefix, nw.ID())); !exists || err != nil {
		t.Fatalf("Network key should have been created.")
	}
	if exists, err := kvStore.Exists(datastore.Key([]string{datastore.EndpointKeyPrefix, nw.ID(), ep.ID()}...)); !exists || err != nil {
		t.Fatalf("Endpoint key should have been created.")
	}
	kvStore.Close()

	// test restore of local store
	testController, err = New(cfgOptions...)
	if err != nil {
		t.Fatalf("Error creating controller: %v", err)
	}
	defer testController.Stop()
	if _, err = testController.NetworkByID(nw.ID()); err != nil {
		t.Fatalf("Error getting network %v", err)
	}
}

// OptionBoltdbWithRandomDBFile function returns a random dir for local store backend
func OptionBoltdbWithRandomDBFile(t *testing.T) config.Option {
	t.Helper()
	tmp := filepath.Join(t.TempDir(), "bolt.db")
	if err := os.WriteFile(tmp, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	return func(c *config.Config) {
		c.Scope.Client.Provider = "boltdb"
		c.Scope.Client.Address = tmp
		c.Scope.Client.Config = &store.Config{Bucket: "testBackend"}
	}
}

func TestMultipleControllersWithSameStore(t *testing.T) {
	cfgOptions := OptionBoltdbWithRandomDBFile(t)
	ctrl1, err := New(cfgOptions)
	if err != nil {
		t.Fatalf("Error new controller: %v", err)
	}
	defer ctrl1.Stop()
	// Use the same boltdb file without closing the previous controller
	ctrl2, err := New(cfgOptions)
	if err != nil {
		t.Fatalf("Local store must support concurrent controllers")
	}
	ctrl2.Stop()
}

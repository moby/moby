package libnetwork

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/docker/libnetwork/config"
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
	ep, err := nw.CreateEndpoint(context.Background(), "newendpoint", []EndpointOption{}...)
	if err != nil {
		t.Fatalf("Error creating endpoint: %v", err)
	}

	nwKVObject := &Network{id: nw.ID()}
	err = testController.store.GetObject(nwKVObject)
	if err != nil {
		t.Errorf("Error when retrieving network key from store: %v", err)
	}
	if !nwKVObject.Exists() {
		t.Errorf("Network key should have been created.")
	}

	epKVObject := &Endpoint{network: nw, id: ep.ID()}
	err = testController.store.GetObject(epKVObject)
	if err != nil {
		t.Errorf("Error when retrieving Endpoint key from store: %v", err)
	}
	if !epKVObject.Exists() {
		t.Errorf("Endpoint key should have been created.")
	}
	testController.Stop()

	// test restore of local store
	testController, err = New(cfgOptions...)
	if err != nil {
		t.Fatalf("Error creating controller: %v", err)
	}
	defer testController.Stop()
	if _, err = testController.NetworkByID(nw.ID()); err != nil {
		t.Errorf("Error getting network %v", err)
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

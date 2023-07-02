package libnetwork

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/libnetwork/config"
	"github.com/docker/docker/libnetwork/datastore"
	store "github.com/docker/docker/libnetwork/internal/kvstore"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/options"
)

func testLocalBackend(t *testing.T, provider, url string, storeConfig *store.Config) {
	cfgOptions := []config.Option{}
	cfgOptions = append(cfgOptions, optionLocalKVProvider(provider))
	cfgOptions = append(cfgOptions, optionLocalKVProviderURL(url))
	cfgOptions = append(cfgOptions, optionLocalKVProviderConfig(storeConfig))

	driverOptions := options.Generic{}
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = driverOptions
	cfgOptions = append(cfgOptions, config.OptionDriverConfig("host", genericOption))

	testController, err := New(cfgOptions...)
	if err != nil {
		t.Fatalf("Error new controller: %v", err)
	}
	defer testController.Stop()
	nw, err := testController.NewNetwork("host", "host", "")
	if err != nil {
		t.Fatalf("Error creating default \"host\" network: %v", err)
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
		optionLocalKVProvider("boltdb")(c)
		optionLocalKVProviderURL(tmp)(c)
		optionLocalKVProviderConfig(&store.Config{Bucket: "testBackend"})(c)
	}
}

// optionLocalKVProvider function returns an option setter for kvstore provider
func optionLocalKVProvider(provider string) config.Option {
	return func(c *config.Config) {
		c.Scope.Client.Provider = strings.TrimSpace(provider)
	}
}

// optionLocalKVProviderURL function returns an option setter for kvstore url
func optionLocalKVProviderURL(url string) config.Option {
	return func(c *config.Config) {
		c.Scope.Client.Address = strings.TrimSpace(url)
	}
}

// optionLocalKVProviderConfig function returns an option setter for kvstore config
func optionLocalKVProviderConfig(cfg *store.Config) config.Option {
	return func(c *config.Config) {
		c.Scope.Client.Config = cfg
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

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
	cfgOptions := []config.Option{}
	cfgOptions = append(cfgOptions, config.OptionLocalKVProvider(provider))
	cfgOptions = append(cfgOptions, config.OptionLocalKVProviderURL(url))
	cfgOptions = append(cfgOptions, config.OptionLocalKVProviderConfig(storeConfig))

	driverOptions := options.Generic{}
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = driverOptions
	cfgOptions = append(cfgOptions, config.OptionDriverConfig("host", genericOption))

	ctrl, err := New(cfgOptions...)
	if err != nil {
		t.Fatalf("Error new controller: %v", err)
	}
	defer ctrl.Stop()
	nw, err := ctrl.NewNetwork("host", "host", "")
	if err != nil {
		t.Fatalf(`Error creating default "host" network: %v`, err)
	}
	ep, err := nw.CreateEndpoint("newendpoint", []EndpointOption{}...)
	if err != nil {
		t.Fatalf("Error creating endpoint: %v", err)
	}
	store := ctrl.getStore().KVStore()
	if exists, err := store.Exists(datastore.Key(datastore.NetworkKeyPrefix, nw.ID())); !exists || err != nil {
		t.Fatalf("Network key should have been created.")
	}
	if exists, err := store.Exists(datastore.Key([]string{datastore.EndpointKeyPrefix, nw.ID(), ep.ID()}...)); !exists || err != nil {
		t.Fatalf("Endpoint key should have been created.")
	}
	store.Close()

	// test restore of local store
	ctrl, err = New(cfgOptions...)
	if err != nil {
		t.Fatalf("Error creating controller: %v", err)
	}
	defer ctrl.Stop()
	if _, err = ctrl.NetworkByID(nw.ID()); err != nil {
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
		config.OptionLocalKVProvider("boltdb")(c)
		config.OptionLocalKVProviderURL(tmp)(c)
		config.OptionLocalKVProviderConfig(&store.Config{Bucket: "testBackend"})(c)
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

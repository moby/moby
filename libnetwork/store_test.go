package libnetwork

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/docker/libkv/store"
	"github.com/docker/libnetwork/config"
	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/options"
)

func TestZooKeeperBackend(t *testing.T) {
	if err := testNewController(t, "zk", "127.0.0.1:2181"); err != nil {
		t.Fatal(err)
	}
}

func testNewController(t *testing.T, provider, url string) error {
	cfgOptions, err := OptionBoltdbWithRandomDBFile()
	if err != nil {
		return err
	}
	cfgOptions = append(cfgOptions, config.OptionKVProvider(provider))
	cfgOptions = append(cfgOptions, config.OptionKVProviderURL(url))
	_, err = New(cfgOptions...)
	return err
}

func TestBoltdbBackend(t *testing.T) {
	defer os.Remove(defaultLocalStoreConfig.Client.Address)
	testLocalBackend(t, "", "", nil)
	defer os.Remove("/tmp/boltdb.db")
	testLocalBackend(t, "boltdb", "/tmp/boltdb.db", &store.Config{Bucket: "testBackend"})
}

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
	nw, err := ctrl.NewNetwork("host", "host")
	if err != nil {
		t.Fatalf("Error creating default \"host\" network: %v", err)
	}
	ep, err := nw.CreateEndpoint("newendpoint", []EndpointOption{}...)
	if err != nil {
		t.Fatalf("Error creating endpoint: %v", err)
	}
	store := ctrl.(*controller).localStore.KVStore()
	if exists, err := store.Exists(datastore.Key(datastore.NetworkKeyPrefix, string(nw.ID()))); !exists || err != nil {
		t.Fatalf("Network key should have been created.")
	}
	if exists, err := store.Exists(datastore.Key([]string{datastore.EndpointKeyPrefix, string(nw.ID()), string(ep.ID())}...)); exists || err != nil {
		t.Fatalf("Endpoint key shouldn't have been created.")
	}
	store.Close()

	// test restore of local store
	ctrl, err = New(cfgOptions...)
	if err != nil {
		t.Fatalf("Error creating controller: %v", err)
	}
	if _, err = ctrl.NetworkByID(nw.ID()); err != nil {
		t.Fatalf("Error getting network %v", err)
	}
}

// OptionBoltdbWithRandomDBFile function returns a random dir for local store backend
func OptionBoltdbWithRandomDBFile() ([]config.Option, error) {
	tmp, err := ioutil.TempFile("", "libnetwork-")
	if err != nil {
		return nil, fmt.Errorf("Error creating temp file: %v", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("Error closing temp file: %v", err)
	}
	cfgOptions := []config.Option{}
	cfgOptions = append(cfgOptions, config.OptionLocalKVProvider("boltdb"))
	cfgOptions = append(cfgOptions, config.OptionLocalKVProviderURL(tmp.Name()))
	cfgOptions = append(cfgOptions, config.OptionLocalKVProviderConfig(&store.Config{Bucket: "testBackend"}))
	return cfgOptions, nil
}

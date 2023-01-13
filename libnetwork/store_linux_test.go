package libnetwork

import (
	"os"
	"testing"

	"github.com/docker/docker/libnetwork/datastore"
	"github.com/docker/libkv/store"
)

func TestBoltdbBackend(t *testing.T) {
	defer os.Remove(datastore.DefaultScope("").Client.Address)
	testLocalBackend(t, "", "", nil)
	defer os.Remove("/tmp/boltdb.db")
	config := &store.Config{Bucket: "testBackend"}
	testLocalBackend(t, "boltdb", "/tmp/boltdb.db", config)
}

func TestNoPersist(t *testing.T) {
	ctrl, err := New(OptionBoltdbWithRandomDBFile(t))
	if err != nil {
		t.Fatalf("Error new controller: %v", err)
	}
	defer ctrl.Stop()
	nw, err := ctrl.NewNetwork("host", "host", "", NetworkOptionPersist(false))
	if err != nil {
		t.Fatalf("Error creating default \"host\" network: %v", err)
	}
	ep, err := nw.CreateEndpoint("newendpoint", []EndpointOption{}...)
	if err != nil {
		t.Fatalf("Error creating endpoint: %v", err)
	}
	store := ctrl.getStore(datastore.LocalScope).KVStore()
	if exists, _ := store.Exists(datastore.Key(datastore.NetworkKeyPrefix, nw.ID())); exists {
		t.Fatalf("Network with persist=false should not be stored in KV Store")
	}
	if exists, _ := store.Exists(datastore.Key([]string{datastore.EndpointKeyPrefix, nw.ID(), ep.ID()}...)); exists {
		t.Fatalf("Endpoint in Network with persist=false should not be stored in KV Store")
	}
	store.Close()
}

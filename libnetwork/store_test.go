package libnetwork

import (
	"context"
	"testing"

	"github.com/docker/docker/libnetwork/config"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/options"
)

func testLocalBackend(t *testing.T, path, bucket string) {
	cfgOptions := []config.Option{
		config.OptionDataDir(path),
		func(c *config.Config) { c.DatastoreBucket = bucket },
		config.OptionDriverConfig("host", map[string]interface{}{
			netlabel.GenericData: options.Generic{},
		}),
	}

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

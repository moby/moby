package libnetwork

import (
	"context"
	"testing"

	"github.com/docker/docker/libnetwork/config"
	"gotest.tools/v3/assert"
)

func TestEndpointStore(t *testing.T) {
	configOption := config.OptionDataDir(t.TempDir())
	c, err := New(configOption)
	assert.NilError(t, err)
	defer c.Stop()

	// Insert a first endpoint
	nw := &Network{id: "testNetwork"}
	ep1 := &Endpoint{network: nw, id: "testEndpoint1"}
	err = c.storeEndpoint(context.Background(), ep1)
	assert.NilError(t, err)

	// Then a second endpoint
	ep2 := &Endpoint{network: nw, id: "testEndpoint2"}
	err = c.storeEndpoint(context.Background(), ep2)
	assert.NilError(t, err)

	// Delete the first endpoint
	err = c.deleteStoredEndpoint(ep1)
	assert.NilError(t, err)

	// Store the second endpoint again
	err = c.storeEndpoint(context.Background(), ep2)
	assert.NilError(t, err)
}

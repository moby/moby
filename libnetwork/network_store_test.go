package libnetwork

import (
	"context"
	"testing"

	"github.com/docker/docker/libnetwork/config"
	"gotest.tools/v3/assert"
)

func TestNetworkStore(t *testing.T) {
	configOption := config.OptionDataDir(t.TempDir())
	c, err := New(configOption)
	assert.NilError(t, err)
	defer c.Stop()

	// Insert a first network
	nw1 := &Network{id: "testNetwork1", configFrom: "config-network"}
	err = c.storeNetwork(context.Background(), nw1)
	assert.NilError(t, err)

	// Then a second network
	nw2 := &Network{id: "testNetwork2"}
	err = c.storeNetwork(context.Background(), nw2)
	assert.NilError(t, err)

	// Delete the first network
	err = c.deleteStoredNetwork(nw1)
	assert.NilError(t, err)

	// Store the second network again
	err = c.storeNetwork(context.Background(), nw2)
	assert.NilError(t, err)
}

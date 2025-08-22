package network

import (
	"context"
	"testing"

	networktypes "github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/integration/internal/network"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func containsNetwork(nws []networktypes.Inspect, networkID string) bool {
	for _, n := range nws {
		if n.ID == networkID {
			return true
		}
	}
	return false
}

// createAmbiguousNetworks creates three networks, of which the second network
// uses a prefix of the first network's ID as name. The third network uses the
// first network's ID as name.
//
// After successful creation, properties of all three networks is returned
func createAmbiguousNetworks(ctx context.Context, t *testing.T, apiClient client.APIClient) (string, string, string) {
	testNet := network.CreateNoError(ctx, t, apiClient, "testNet")
	idPrefixNet := network.CreateNoError(ctx, t, apiClient, testNet[:12])
	fullIDNet := network.CreateNoError(ctx, t, apiClient, testNet)

	nws, err := apiClient.NetworkList(ctx, client.NetworkListOptions{})
	assert.NilError(t, err)

	assert.Check(t, is.Equal(true, containsNetwork(nws, testNet)), "failed to create network testNet")
	assert.Check(t, is.Equal(true, containsNetwork(nws, idPrefixNet)), "failed to create network idPrefixNet")
	assert.Check(t, is.Equal(true, containsNetwork(nws, fullIDNet)), "failed to create network fullIDNet")
	return testNet, idPrefixNet, fullIDNet
}

// TestNetworkCreateDelete tests creation and deletion of a network.
func TestNetworkCreateDelete(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	netName := "testnetwork_" + t.Name()
	network.CreateNoError(ctx, t, apiClient, netName)
	assert.Check(t, IsNetworkAvailable(ctx, apiClient, netName))

	// delete the network and make sure it is deleted
	err := apiClient.NetworkRemove(ctx, netName)
	assert.NilError(t, err)
	assert.Check(t, IsNetworkNotAvailable(ctx, apiClient, netName))
}

// TestDockerNetworkDeletePreferID tests that if a network with a name
// equal to another network's ID exists, the Network with the given
// ID is removed, and not the network with the given name.
func TestDockerNetworkDeletePreferID(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows",
		"FIXME. Windows doesn't run DinD and uses networks shared between control daemon and daemon under test")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	testNet, idPrefixNet, fullIDNet := createAmbiguousNetworks(ctx, t, apiClient)

	// Delete the network using a prefix of the first network's ID as name.
	// This should the network name with the id-prefix, not the original network.
	err := apiClient.NetworkRemove(ctx, testNet[:12])
	assert.NilError(t, err)

	// Delete the network using networkID. This should remove the original
	// network, not the network with the name equal to the networkID
	err = apiClient.NetworkRemove(ctx, testNet)
	assert.NilError(t, err)

	// networks "testNet" and "idPrefixNet" should be removed, but "fullIDNet" should still exist
	nws, err := apiClient.NetworkList(ctx, client.NetworkListOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(false, containsNetwork(nws, testNet)), "Network testNet not removed")
	assert.Check(t, is.Equal(false, containsNetwork(nws, idPrefixNet)), "Network idPrefixNet not removed")
	assert.Check(t, is.Equal(true, containsNetwork(nws, fullIDNet)), "Network fullIDNet not found")
}

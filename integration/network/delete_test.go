package network // import "github.com/docker/docker/integration/network"

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/versions"
	dclient "github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/network"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func containsNetwork(nws []types.NetworkResource, networkID string) bool {
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
func createAmbiguousNetworks(ctx context.Context, t *testing.T, client dclient.APIClient) (string, string, string) {
	testNet := network.CreateNoError(ctx, t, client, "testNet")
	idPrefixNet := network.CreateNoError(ctx, t, client, testNet[:12])
	fullIDNet := network.CreateNoError(ctx, t, client, testNet)

	nws, err := client.NetworkList(ctx, types.NetworkListOptions{})
	assert.NilError(t, err)

	assert.Check(t, is.Equal(true, containsNetwork(nws, testNet)), "failed to create network testNet")
	assert.Check(t, is.Equal(true, containsNetwork(nws, idPrefixNet)), "failed to create network idPrefixNet")
	assert.Check(t, is.Equal(true, containsNetwork(nws, fullIDNet)), "failed to create network fullIDNet")
	return testNet, idPrefixNet, fullIDNet
}

// TestNetworkCreateDelete tests creation and deletion of a network.
func TestNetworkCreateDelete(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	netName := "testnetwork_" + t.Name()
	network.CreateNoError(ctx, t, client, netName,
		network.WithCheckDuplicate(),
	)
	assert.Check(t, IsNetworkAvailable(client, netName))

	// delete the network and make sure it is deleted
	err := client.NetworkRemove(ctx, netName)
	assert.NilError(t, err)
	assert.Check(t, IsNetworkNotAvailable(client, netName))
}

// TestDockerNetworkDeletePreferID tests that if a network with a name
// equal to another network's ID exists, the Network with the given
// ID is removed, and not the network with the given name.
func TestDockerNetworkDeletePreferID(t *testing.T) {
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.34"), "broken in earlier versions")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows",
		"FIXME. Windows doesn't run DinD and uses networks shared between control daemon and daemon under test")
	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()
	testNet, idPrefixNet, fullIDNet := createAmbiguousNetworks(ctx, t, client)

	// Delete the network using a prefix of the first network's ID as name.
	// This should the network name with the id-prefix, not the original network.
	err := client.NetworkRemove(ctx, testNet[:12])
	assert.NilError(t, err)

	// Delete the network using networkID. This should remove the original
	// network, not the network with the name equal to the networkID
	err = client.NetworkRemove(ctx, testNet)
	assert.NilError(t, err)

	// networks "testNet" and "idPrefixNet" should be removed, but "fullIDNet" should still exist
	nws, err := client.NetworkList(ctx, types.NetworkListOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(false, containsNetwork(nws, testNet)), "Network testNet not removed")
	assert.Check(t, is.Equal(false, containsNetwork(nws, idPrefixNet)), "Network idPrefixNet not removed")
	assert.Check(t, is.Equal(true, containsNetwork(nws, fullIDNet)), "Network fullIDNet not found")
}

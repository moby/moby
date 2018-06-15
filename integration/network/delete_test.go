package network // import "github.com/docker/docker/integration/network"

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/integration/internal/network"
	"github.com/docker/docker/internal/test/request"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	"gotest.tools/skip"
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
func createAmbiguousNetworks(t *testing.T) (string, string, string) {
	client := request.NewAPIClient(t)
	ctx := context.Background()

	testNet := network.CreateNoError(t, ctx, client, "testNet")
	idPrefixNet := network.CreateNoError(t, ctx, client, testNet[:12])
	fullIDNet := network.CreateNoError(t, ctx, client, testNet)

	nws, err := client.NetworkList(ctx, types.NetworkListOptions{})
	assert.NilError(t, err)

	assert.Check(t, is.Equal(true, containsNetwork(nws, testNet)), "failed to create network testNet")
	assert.Check(t, is.Equal(true, containsNetwork(nws, idPrefixNet)), "failed to create network idPrefixNet")
	assert.Check(t, is.Equal(true, containsNetwork(nws, fullIDNet)), "failed to create network fullIDNet")
	return testNet, idPrefixNet, fullIDNet
}

func TestNetworkCreateDelete(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	netName := "testnetwork_" + t.Name()
	network.CreateNoError(t, ctx, client, netName,
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
	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()
	testNet, idPrefixNet, fullIDNet := createAmbiguousNetworks(t)

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

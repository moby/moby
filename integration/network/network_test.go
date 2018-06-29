package network // import "github.com/docker/docker/integration/network"

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/docker/docker/api/types"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/network"
	"github.com/docker/docker/integration/internal/swarm"
	"github.com/docker/docker/internal/test/request"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	"gotest.tools/skip"
)

func TestNetworkGetDefaults(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	defer setupTest(t)()
	client := request.NewAPIClient(t)

	// By default docker daemon creates 3 networks. check if they are present
	defaults := []string{"bridge", "host", "none"}
	for _, nn := range defaults {
		assert.Check(t, IsNetworkAvailable(client, nn))
	}
}

func TestNetworkCreateCheckDuplicate(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.34"))
	defer setupTest(t)()
	client := request.NewAPIClient(t)
	name := "testcheckduplicate_" + t.Name()

	// Creating a new network first
	network.CreateNoError(t, context.Background(), client, name)
	assert.Check(t, IsNetworkAvailable(client, name))

	// Creating another network with same name and CheckDuplicate must fail
	_, err := network.Create(context.Background(), client, name,
		network.WithCheckDuplicate(),
	)
	assert.Check(t, is.ErrorContains(err, "network with name "+name+" already exists"))

	// Creating another network with same name and CheckDuplicate not set must succeed
	network.CreateNoError(t, context.Background(), client, name)
}

func TestNetworkConnectDisconnect(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	defer setupTest(t)()
	client := request.NewAPIClient(t)
	// Create test network
	netName := "testnetwork_" + t.Name()
	netID := network.CreateNoError(t, context.Background(), client, netName)
	nr, err := client.NetworkInspect(context.Background(), netID, types.NetworkInspectOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(netName, nr.Name))
	assert.Check(t, is.Equal(netID, nr.ID))
	assert.Check(t, is.Equal(0, len(nr.Containers)))

	containerID := container.Run(t, context.Background(), client, container.WithName("test_"+t.Name()),
		container.WithCmd("top"),
	)

	// connect the container to the test network
	err = client.NetworkConnect(context.Background(), netID, containerID, &networktypes.EndpointSettings{})
	assert.NilError(t, err)

	// inspect the network to make sure container is connected
	nr, err = client.NetworkInspect(context.Background(), netID, types.NetworkInspectOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(1, len(nr.Containers)))
	_, ok := nr.Containers[containerID]
	assert.Check(t, ok)

	// check if container IP matches network inspect
	ip, _, err := net.ParseCIDR(nr.Containers[containerID].IPv4Address)
	assert.NilError(t, err)
	containerIP := FindContainerIP(t, client, containerID, netName)
	assert.Check(t, is.Equal(containerIP, ip.String()))

	// disconnect container from the network
	err = client.NetworkDisconnect(context.Background(), netID, containerID, false)
	assert.NilError(t, err)
	nr, err = client.NetworkInspect(context.Background(), netID, types.NetworkInspectOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(netName, nr.Name))
	assert.Check(t, is.Equal(0, len(nr.Containers)))

	// delete the network
	err = client.NetworkRemove(context.Background(), netName)
	assert.NilError(t, err)
}

func TestNetworkIPAMMultipleBridgeNetworks(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.32"))
	defer setupTest(t)()
	client := request.NewAPIClient(t)

	netName := "test0_" + t.Name()
	// test0 bridge network
	id0 := network.CreateNoError(t, context.Background(), client, netName,
		network.WithDriver("bridge"),
		network.WithIPAM("default", "192.178.0.0/16", "192.178.138.100", "192.178.128.0/17"),
	)
	assert.Check(t, IsNetworkAvailable(client, netName))

	netName1 := "test1_" + t.Name()
	// test1 bridge network overlaps with test0
	config1 := []func(*types.NetworkCreate){
		network.WithDriver("bridge"),
		network.WithIPAM("default", "192.178.128.0/17", "192.178.128.1", ""),
	}

	_, err := network.Create(context.Background(), client, netName1, config1...)
	assert.Check(t, err != nil)
	assert.Check(t, IsNetworkNotAvailable(client, netName1))

	netName2 := "test2_" + t.Name()
	// test2 bridge network does not overlap
	network.CreateNoError(t, context.Background(), client, netName2,
		network.WithDriver("bridge"),
		network.WithIPAM("default", "192.169.0.0/16", "192.169.100.100", ""),
	)
	assert.Check(t, IsNetworkAvailable(client, netName2))

	// remove test0 and retry to create test1
	err = client.NetworkRemove(context.Background(), id0)
	assert.NilError(t, err)

	network.CreateNoError(t, context.Background(), client, netName1, config1...)
	assert.Check(t, IsNetworkAvailable(client, netName1))

	// for networks w/o ipam specified, docker will choose proper non-overlapping subnets
	network.CreateNoError(t, context.Background(), client, "test3_"+t.Name())
	assert.Check(t, IsNetworkAvailable(client, "test3_"+t.Name()))
	network.CreateNoError(t, context.Background(), client, "test4_"+t.Name())
	assert.Check(t, IsNetworkAvailable(client, "test4_"+t.Name()))
	network.CreateNoError(t, context.Background(), client, "test5_"+t.Name())
	assert.Check(t, IsNetworkAvailable(client, "test5_"+t.Name()))

	for i := 1; i < 6; i++ {
		client.NetworkRemove(context.Background(), fmt.Sprintf("test%d_"+t.Name(), i))
	}
}

func TestCreateDeletePredefinedNetworks(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, !swarm.IsSwarmInactive(testEnv))
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.34"))
	defer setupTest(t)()
	client := request.NewAPIClient(t)

	createDeletePredefinedNetwork(t, client, "bridge")
	createDeletePredefinedNetwork(t, client, "none")
	createDeletePredefinedNetwork(t, client, "host")
}

func createDeletePredefinedNetwork(t *testing.T, client client.NetworkAPIClient, networkName string) {
	_, err := network.Create(context.Background(), client, networkName, network.WithCheckDuplicate())
	assert.Check(t, err != nil)
	err = client.NetworkRemove(context.Background(), networkName)
	assert.Check(t, err != nil)
}

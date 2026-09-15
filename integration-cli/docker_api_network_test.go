package main

import (
	"encoding/json"
	"net/http"
	"net/netip"
	"strings"
	"testing"

	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/integration-cli/cli"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/request"
	"gotest.tools/v3/assert"
)

func (s *DockerAPISuite) TestAPINetworkInspectBridge(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	// Inspect default bridge network
	nr := getNetworkResource(c, "bridge")
	assert.Equal(c, nr.Name, "bridge")

	// run a container and attach it to the default bridge network
	out := cli.DockerCmd(c, "run", "-d", "--name", "test", "busybox", "top").Stdout()
	containerID := strings.TrimSpace(out)
	containerIP := findContainerIP(c, "test", "bridge")

	// inspect default bridge network again and make sure the container is connected
	nr = getNetworkResource(c, nr.ID)
	assert.Equal(c, nr.Driver, "bridge")
	assert.Equal(c, nr.Scope, "local")
	assert.Equal(c, nr.Internal, false)
	assert.Equal(c, nr.EnableIPv6, false)
	assert.Equal(c, nr.IPAM.Driver, "default")
	_, ok := nr.Containers[containerID]
	assert.Assert(c, ok)

	assert.Equal(c, nr.Containers[containerID].IPv4Address.Addr().String(), containerIP)
}

func (s *DockerAPISuite) TestAPINetworkConnectDisconnect(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	// Create test network
	name := "testnetwork"
	config := network.CreateRequest{
		Name: name,
	}
	id := createNetwork(c, config, http.StatusCreated)
	nr := getNetworkResource(c, id)
	assert.Equal(c, nr.Name, name)
	assert.Equal(c, nr.ID, id)
	assert.Equal(c, len(nr.Containers), 0)

	// run a container
	out := cli.DockerCmd(c, "run", "-d", "--name", "test", "busybox", "top").Stdout()
	containerID := strings.TrimSpace(out)

	// connect the container to the test network
	connectNetwork(c, nr.ID, containerID)

	// inspect the network to make sure container is connected
	nr = getNetworkResource(c, nr.ID)
	assert.Equal(c, len(nr.Containers), 1)
	_, ok := nr.Containers[containerID]
	assert.Assert(c, ok)

	// check if container IP matches network inspect
	containerIP := findContainerIP(c, "test", "testnetwork")
	assert.Equal(c, nr.Containers[containerID].IPv4Address.Addr().String(), containerIP)

	// disconnect container from the network
	disconnectNetwork(c, nr.ID, containerID)
	nr = getNetworkResource(c, nr.ID)
	assert.Equal(c, nr.Name, name)
	assert.Equal(c, len(nr.Containers), 0)

	// delete the network
	deleteNetwork(c, nr.ID, true)
}

func (s *DockerAPISuite) TestAPICreateDeletePredefinedNetworks(c *testing.T) {
	testRequires(c, DaemonIsLinux, SwarmInactive)
	createDeletePredefinedNetwork(c, "bridge")
	createDeletePredefinedNetwork(c, "none")
	createDeletePredefinedNetwork(c, "host")
}

func createDeletePredefinedNetwork(t *testing.T, name string) {
	// Create pre-defined network
	config := network.CreateRequest{Name: name}
	expectedStatus := http.StatusForbidden
	createNetwork(t, config, expectedStatus)
	deleteNetwork(t, name, false)
}

func isNetworkAvailable(t *testing.T, name string) bool {
	resp, body, err := request.Get(testutil.GetContext(t), "/networks")
	assert.NilError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	var nJSON []network.Inspect
	err = json.NewDecoder(body).Decode(&nJSON)
	assert.NilError(t, err)

	for _, n := range nJSON {
		if n.Name == name {
			return true
		}
	}
	return false
}

func getNetworkResource(t *testing.T, id string) *network.Inspect {
	_, obj, err := request.Get(testutil.GetContext(t), "/networks/"+id)
	assert.NilError(t, err)

	nr := network.Inspect{}
	err = json.NewDecoder(obj).Decode(&nr)
	assert.NilError(t, err)

	return &nr
}

func createNetwork(t *testing.T, config network.CreateRequest, expectedStatusCode int) string {
	t.Helper()

	resp, body, err := request.Post(testutil.GetContext(t), "/networks/create", request.JSONBody(config))
	assert.NilError(t, err)
	defer resp.Body.Close()

	if expectedStatusCode >= 0 {
		assert.Equal(t, resp.StatusCode, expectedStatusCode)
	} else {
		assert.Assert(t, resp.StatusCode != -expectedStatusCode)
	}

	if expectedStatusCode == http.StatusCreated || expectedStatusCode < 0 {
		var nr network.CreateResponse
		err = json.NewDecoder(body).Decode(&nr)
		assert.NilError(t, err)

		return nr.ID
	}
	return ""
}

func connectNetwork(t *testing.T, nid, cid string) {
	resp, _, err := request.Post(testutil.GetContext(t), "/networks/"+nid+"/connect", request.JSONBody(network.ConnectRequest{
		Container: cid,
	}))
	assert.NilError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
}

func disconnectNetwork(t *testing.T, nid, cid string) {
	config := client.NetworkDisconnectOptions{
		Container: cid,
	}

	resp, _, err := request.Post(testutil.GetContext(t), "/networks/"+nid+"/disconnect", request.JSONBody(config))
	assert.NilError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
}

func deleteNetwork(t *testing.T, id string, shouldSucceed bool) {
	resp, _, err := request.Delete(testutil.GetContext(t), "/networks/"+id)
	assert.NilError(t, err)
	defer resp.Body.Close()
	if !shouldSucceed {
		assert.Assert(t, resp.StatusCode != http.StatusOK)
		return
	}
	assert.Equal(t, resp.StatusCode, http.StatusNoContent)
}

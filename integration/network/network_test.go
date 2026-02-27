package network

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	networktypes "github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"

	"github.com/moby/moby/v2/integration/internal/container"
	"github.com/moby/moby/v2/integration/internal/network"
	"github.com/moby/moby/v2/integration/internal/swarm"

	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/request"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"

	is "gotest.tools/v3/assert/cmp"
)

// TestNetworkInvalidJSON tests that POST endpoints that expect a body return
// the correct error when sending invalid JSON requests.
func TestNetworkInvalidJSON(t *testing.T) {
	ctx := setupTest(t)

	// POST endpoints that accept / expect a JSON body;
	endpoints := []string{
		"/networks/create",
		"/networks/bridge/connect",
		"/networks/bridge/disconnect",
	}

	for _, ep := range endpoints {
		t.Run(ep[1:], func(t *testing.T) {
			t.Parallel()
			ctx := testutil.StartSpan(ctx, t)

			t.Run("invalid content type", func(t *testing.T) {
				ctx := testutil.StartSpan(ctx, t)
				res, body, err := request.Post(ctx, ep, request.RawString("{}"), request.ContentType("text/plain"))
				assert.NilError(t, err)
				assert.Check(t, is.Equal(res.StatusCode, http.StatusBadRequest))

				buf, err := request.ReadBody(body)
				assert.NilError(t, err)
				assert.Check(t, is.Contains(string(buf), "unsupported Content-Type header (text/plain): must be 'application/json'"))
			})

			t.Run("invalid JSON", func(t *testing.T) {
				ctx := testutil.StartSpan(ctx, t)
				res, body, err := request.Post(ctx, ep, request.RawString("{invalid json"), request.JSON)
				assert.NilError(t, err)
				assert.Check(t, is.Equal(res.StatusCode, http.StatusBadRequest))

				buf, err := request.ReadBody(body)
				assert.NilError(t, err)
				assert.Check(t, is.Contains(string(buf), "invalid JSON: invalid character 'i' looking for beginning of object key string"))
			})

			t.Run("extra content after JSON", func(t *testing.T) {
				ctx := testutil.StartSpan(ctx, t)
				res, body, err := request.Post(ctx, ep, request.RawString(`{} trailing content`), request.JSON)
				assert.NilError(t, err)
				assert.Check(t, is.Equal(res.StatusCode, http.StatusBadRequest))

				buf, err := request.ReadBody(body)
				assert.NilError(t, err)
				assert.Check(t, is.Contains(string(buf), "unexpected content after JSON"))
			})

			t.Run("empty body", func(t *testing.T) {
				ctx := testutil.StartSpan(ctx, t)
				// empty body should not produce an 500 internal server error, or
				// any 5XX error (this is assuming the request does not produce
				// an internal server error for another reason, but it shouldn't)
				res, _, err := request.Post(ctx, ep, request.RawString(``), request.JSON)
				assert.NilError(t, err)
				assert.Check(t, res.StatusCode < http.StatusInternalServerError)
			})
		})
	}
}

// TestNetworkList verifies that /networks returns a list of networks either
// with, or without a trailing slash (/networks/). Regression test for https://github.com/moby/moby/issues/24595
func TestNetworkList(t *testing.T) {
	ctx := setupTest(t)

	endpoints := []string{
		"/networks",
		"/networks/",
	}

	for _, ep := range endpoints {
		t.Run(ep, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)
			t.Parallel()

			res, body, err := request.Get(ctx, ep, request.JSON)
			assert.NilError(t, err)
			assert.Equal(t, res.StatusCode, http.StatusOK)

			buf, err := request.ReadBody(body)
			assert.NilError(t, err)
			var nws []networktypes.Inspect
			err = json.Unmarshal(buf, &nws)
			assert.NilError(t, err)
			assert.Assert(t, len(nws) > 0)
		})
	}
}

func TestAPINetworkGetDefaults(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	defaults := []string{"bridge", "host", "none"}
	if testEnv.DaemonInfo.OSType == "windows" {
		defaults = []string{"nat", "none"}
	}

	for _, netName := range defaults {
		assert.Assert(t, IsNetworkAvailable(ctx, apiClient, netName))
	}
}

func TestAPINetworkFilter(t *testing.T) {
	networkName := "bridge"
	if testEnv.DaemonInfo.OSType == "windows" {
		networkName = "nat"
	}

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	res, err := apiClient.NetworkList(ctx, client.NetworkListOptions{
		Filters: make(client.Filters).Add("name", networkName),
	})

	assert.NilError(t, err)

	found := false
	for _, nw := range res.Items {
		if nw.Name == networkName {
			found = true
		}
	}
	assert.Assert(t, found, fmt.Sprintf("%s is not found", networkName))
}

func TestNetworkInspectWithScope(t *testing.T) {
	ctx := setupTest(t)

	d := swarm.NewSwarm(ctx, t, testEnv)
	defer d.Stop(t)

	cli := d.NewClientT(t) // IMPORTANT: talk to swarm daemon

	name := "test-scoped-network"
	create, err := cli.NetworkCreate(ctx, name, client.NetworkCreateOptions{Driver: "overlay"})
	assert.NilError(t, err)

	inspect, err := cli.NetworkInspect(ctx, name, client.NetworkInspectOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal("swarm", inspect.Network.Scope))
	assert.Check(t, is.Equal(create.ID, inspect.Network.ID))

	_, err = cli.NetworkInspect(ctx, name, client.NetworkInspectOptions{Scope: "local"})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
}

func TestBridgeAndCustomNetworkAttach(t *testing.T) {
	skip.If(t, testEnv.RuntimeIsWindowsContainerd(),
		"Skipping test: fails on Containerd due to unsupported platform request error during NetworkConnect operations")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	const name = "testnetwork"
	const containerName = "test-container"

	network.CreateNoError(ctx, t, apiClient, name)
	defer network.RemoveNoError(ctx, t, apiClient, name)

	// verifying the new network doesn't attach any container
	nr, err := apiClient.NetworkInspect(ctx, name, client.NetworkInspectOptions{})
	assert.NilError(t, err)
	assert.Equal(t, nr.Network.Name, name)
	assert.Equal(t, len(nr.Network.Containers), 0)

	// starting a container without attaching to the network created above
	id := container.Run(ctx, t, apiClient, container.WithName(containerName))
	defer func() {
		_, _ = apiClient.ContainerRemove(ctx, id, client.ContainerRemoveOptions{})
	}()

	// attaching the custom network to the above running conainer .
	_, err = apiClient.NetworkConnect(ctx, name, client.NetworkConnectOptions{
		Container: id,
	})
	assert.NilError(t, err)

	// verify the container is listed in the network inspect output
	nr, err = apiClient.NetworkInspect(ctx, name, client.NetworkInspectOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(nr.Network.Containers), 1)

	_, exists := nr.Network.Containers[id]
	assert.Assert(t, exists)

	// inspecting the container and extracting the IP.
	contInspect, err := apiClient.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	assert.NilError(t, err)

	var containerIP string
	for n, settings := range contInspect.Container.NetworkSettings.Networks {
		if n == name {
			containerIP = settings.IPAddress.String()
			break
		}
	}
	assert.Assert(t, containerIP != "")
	assert.Equal(t, nr.Network.Containers[id].IPv4Address.Addr().String(), containerIP)

	// detaching the container from the custom network
	_, err = apiClient.NetworkDisconnect(ctx, name, client.NetworkDisconnectOptions{
		Container: id,
	})
	assert.NilError(t, err)

	// verifying no container is attached to the network.
	nr, err = apiClient.NetworkInspect(ctx, name, client.NetworkInspectOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(nr.Network.Containers), 0)
}

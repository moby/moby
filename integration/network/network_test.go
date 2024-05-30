package network // import "github.com/docker/docker/integration/network"

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	ntypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/network"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/daemon"
	"github.com/docker/docker/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/icmd"
	"gotest.tools/v3/skip"
)

func TestRunContainerWithBridgeNone(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot start daemon on remote test run")
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, testEnv.IsUserNamespace)
	skip.If(t, testEnv.IsRootless, "rootless mode has different view of network")

	ctx := testutil.StartSpan(baseContext, t)

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t, "-b", "none")
	defer d.Stop(t)

	c := d.NewClientT(t)

	id1 := container.Run(ctx, t, c)
	defer c.ContainerRemove(ctx, id1, containertypes.RemoveOptions{Force: true})

	result, err := container.Exec(ctx, c, id1, []string{"ip", "l"})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(false, strings.Contains(result.Combined(), "eth0")), "There shouldn't be eth0 in container in default(bridge) mode when bridge network is disabled")

	id2 := container.Run(ctx, t, c, container.WithNetworkMode("bridge"))
	defer c.ContainerRemove(ctx, id2, containertypes.RemoveOptions{Force: true})

	result, err = container.Exec(ctx, c, id2, []string{"ip", "l"})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(false, strings.Contains(result.Combined(), "eth0")), "There shouldn't be eth0 in container in bridge mode when bridge network is disabled")

	nsCommand := "ls -l /proc/self/ns/net | awk -F '->' '{print $2}'"
	cmd := exec.Command("sh", "-c", nsCommand)
	stdout := bytes.NewBuffer(nil)
	cmd.Stdout = stdout
	err = cmd.Run()
	assert.NilError(t, err, "Failed to get current process network namespace: %+v", err)

	id3 := container.Run(ctx, t, c, container.WithNetworkMode("host"))
	defer c.ContainerRemove(ctx, id3, containertypes.RemoveOptions{Force: true})

	result, err = container.Exec(ctx, c, id3, []string{"sh", "-c", nsCommand})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(stdout.String(), result.Combined()), "The network namespace of container should be the same with host when --net=host and bridge network is disabled")
}

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
		ep := ep
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
		ep := ep
		t.Run(ep, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)
			t.Parallel()

			res, body, err := request.Get(ctx, ep, request.JSON)
			assert.NilError(t, err)
			assert.Equal(t, res.StatusCode, http.StatusOK)

			buf, err := request.ReadBody(body)
			assert.NilError(t, err)
			var nws []types.NetworkResource
			err = json.Unmarshal(buf, &nws)
			assert.NilError(t, err)
			assert.Assert(t, len(nws) > 0)
		})
	}
}

func TestHostIPv4BridgeLabel(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.IsRootless, "rootless mode has different view of network")
	ctx := testutil.StartSpan(baseContext, t)

	d := daemon.New(t)
	d.Start(t)
	defer d.Stop(t)
	c := d.NewClientT(t)
	defer c.Close()

	ipv4SNATAddr := "172.0.0.172"
	// Create a bridge network with --opt com.docker.network.host_ipv4=172.0.0.172
	bridgeName := "hostIPv4Bridge"
	network.CreateNoError(ctx, t, c, bridgeName,
		network.WithDriver("bridge"),
		network.WithOption("com.docker.network.host_ipv4", ipv4SNATAddr),
		network.WithOption("com.docker.network.bridge.name", bridgeName),
	)
	out, err := c.NetworkInspect(ctx, bridgeName, ntypes.InspectOptions{Verbose: true})
	assert.NilError(t, err)
	assert.Assert(t, len(out.IPAM.Config) > 0)
	// Make sure the SNAT rule exists
	testutil.RunCommand(ctx, "iptables", "-t", "nat", "-C", "POSTROUTING", "-s", out.IPAM.Config[0].Subnet, "!", "-o", bridgeName, "-j", "SNAT", "--to-source", ipv4SNATAddr).Assert(t, icmd.Success)
}

func TestDefaultNetworkOpts(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.IsRootless, "rootless mode has different view of network")
	ctx := testutil.StartSpan(baseContext, t)

	tests := []struct {
		name       string
		mtu        int
		configFrom bool
		args       []string
	}{
		{
			name: "default value",
			mtu:  1500,
			args: []string{},
		},
		{
			name: "cmdline value",
			mtu:  1234,
			args: []string{"--default-network-opt", "bridge=com.docker.network.driver.mtu=1234"},
		},
		{
			name:       "config-from value",
			configFrom: true,
			mtu:        1233,
			args:       []string{"--default-network-opt", "bridge=com.docker.network.driver.mtu=1234"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)
			d := daemon.New(t)
			d.StartWithBusybox(ctx, t, tc.args...)
			defer d.Stop(t)
			c := d.NewClientT(t)
			defer c.Close()

			if tc.configFrom {
				// Create a new network config
				network.CreateNoError(ctx, t, c, "from-net", func(create *types.NetworkCreate) {
					create.ConfigOnly = true
					create.Options = map[string]string{
						"com.docker.network.driver.mtu": fmt.Sprint(tc.mtu),
					}
				})
				defer c.NetworkRemove(ctx, "from-net")
			}

			// Create a new network
			networkName := "testnet"
			networkId := network.CreateNoError(ctx, t, c, networkName, func(create *types.NetworkCreate) {
				if tc.configFrom {
					create.ConfigFrom = &ntypes.ConfigReference{
						Network: "from-net",
					}
				}
			})
			defer c.NetworkRemove(ctx, networkName)

			// Check the MTU of the bridge itself, before any devices are connected. (The
			// bridge's MTU will be set to the minimum MTU of anything connected to it, but
			// it's set explicitly on the bridge anyway - so it doesn't look like the option
			// was ignored.)
			cmd := exec.Command("ip", "link", "show", "br-"+networkId[:12])
			output, err := cmd.CombinedOutput()
			assert.NilError(t, err)
			assert.Check(t, is.Contains(string(output), fmt.Sprintf(" mtu %d ", tc.mtu)), "Bridge MTU should have been set to %d", tc.mtu)

			// Start a container to inspect the MTU of its network interface
			id1 := container.Run(ctx, t, c, container.WithNetworkMode(networkName))
			defer c.ContainerRemove(ctx, id1, containertypes.RemoveOptions{Force: true})

			result, err := container.Exec(ctx, c, id1, []string{"ip", "l", "show", "eth0"})
			assert.NilError(t, err)
			assert.Check(t, is.Contains(result.Combined(), fmt.Sprintf(" mtu %d ", tc.mtu)), "Network MTU should have been set to %d", tc.mtu)
		})
	}
}

func TestForbidDuplicateNetworkNames(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

	ctx := testutil.StartSpan(baseContext, t)

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	network.CreateNoError(ctx, t, c, "testnet")

	_, err := c.NetworkCreate(ctx, "testnet", types.NetworkCreate{})
	assert.Error(t, err, "Error response from daemon: network with name testnet already exists", "2nd NetworkCreate call should have failed")
}

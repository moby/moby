package networking

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/network"
	"github.com/docker/docker/libnetwork/drivers/bridge"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/daemon"
	"github.com/docker/go-connections/nat"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestDisableNAT(t *testing.T) {
	ctx := setupTest(t)
	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	testcases := []struct {
		name       string
		gwMode4    string
		gwMode6    string
		expPortMap nat.PortMap
	}{
		{
			name: "defaults",
			expPortMap: nat.PortMap{
				"80/tcp": []nat.PortBinding{
					{HostIP: "0.0.0.0", HostPort: "8080"},
					{HostIP: "::", HostPort: "8080"},
				},
			},
		},
		{
			name:    "nat4 routed6",
			gwMode4: "nat",
			gwMode6: "routed",
			expPortMap: nat.PortMap{
				"80/tcp": []nat.PortBinding{
					{HostIP: "0.0.0.0", HostPort: "8080"},
					{HostIP: "::", HostPort: ""},
				},
			},
		},
		{
			name:    "nat6 routed4",
			gwMode4: "routed",
			gwMode6: "nat",
			expPortMap: nat.PortMap{
				"80/tcp": []nat.PortBinding{
					{HostIP: "0.0.0.0", HostPort: ""},
					{HostIP: "::", HostPort: "8080"},
				},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)

			const netName = "testnet"
			nwOpts := []func(options *networktypes.CreateOptions){
				network.WithIPv6(),
				network.WithIPAM("fd2a:a2c3:4448::/64", "fd2a:a2c3:4448::1"),
			}
			if tc.gwMode4 != "" {
				nwOpts = append(nwOpts, network.WithOption(bridge.IPv4GatewayMode, tc.gwMode4))
			}
			if tc.gwMode6 != "" {
				nwOpts = append(nwOpts, network.WithOption(bridge.IPv6GatewayMode, tc.gwMode6))
			}
			network.CreateNoError(ctx, t, c, netName, nwOpts...)
			defer network.RemoveNoError(ctx, t, c, netName)

			id := container.Run(ctx, t, c,
				container.WithNetworkMode(netName),
				container.WithExposedPorts("80/tcp"),
				container.WithPortMap(nat.PortMap{"80/tcp": {{HostPort: "8080"}}}),
			)
			defer c.ContainerRemove(ctx, id, containertypes.RemoveOptions{Force: true})

			inspect := container.Inspect(ctx, t, c, id)
			assert.Check(t, is.DeepEqual(inspect.NetworkSettings.Ports, tc.expPortMap))
		})
	}
}

// Check that a container on one network can reach a service in a container on
// another network, via a mapped port on the host.
func TestPortMappedHairpin(t *testing.T) {
	skip.If(t, testEnv.IsRootless)

	ctx := setupTest(t)
	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)
	c := d.NewClientT(t)
	defer c.Close()

	// Find an address on the test host.
	conn, err := net.Dial("tcp4", "hub.docker.com:80")
	assert.NilError(t, err)
	hostAddr := conn.LocalAddr().(*net.TCPAddr).IP.String()
	conn.Close()

	const serverNetName = "servernet"
	network.CreateNoError(ctx, t, c, serverNetName)
	defer network.RemoveNoError(ctx, t, c, serverNetName)
	const clientNetName = "clientnet"
	network.CreateNoError(ctx, t, c, clientNetName)
	defer network.RemoveNoError(ctx, t, c, clientNetName)

	serverId := container.Run(ctx, t, c,
		container.WithNetworkMode(serverNetName),
		container.WithExposedPorts("80"),
		container.WithPortMap(nat.PortMap{"80": {{HostIP: "0.0.0.0"}}}),
		container.WithCmd("httpd", "-f"),
	)
	defer c.ContainerRemove(ctx, serverId, containertypes.RemoveOptions{Force: true})

	inspect := container.Inspect(ctx, t, c, serverId)
	hostPort := inspect.NetworkSettings.Ports["80/tcp"][0].HostPort

	clientCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	res := container.RunAttach(clientCtx, t, c,
		container.WithNetworkMode(clientNetName),
		container.WithCmd("wget", "http://"+hostAddr+":"+hostPort),
	)
	defer c.ContainerRemove(ctx, res.ContainerID, containertypes.RemoveOptions{Force: true})
	assert.Check(t, is.Contains(res.Stderr.String(), "404 Not Found"))
}

// Check that a container on an IPv4-only network can have a port mapping
// from a specific IPv6 host address (using docker-proxy).
// Regression test for https://github.com/moby/moby/issues/48067 (which
// is about incorrectly reporting this as invalid config).
func TestProxy4To6(t *testing.T) {
	skip.If(t, testEnv.IsRootless)

	ctx := setupTest(t)
	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	const netName = "ipv4net"
	network.CreateNoError(ctx, t, c, netName)

	serverId := container.Run(ctx, t, c,
		container.WithNetworkMode(netName),
		container.WithExposedPorts("80"),
		container.WithPortMap(nat.PortMap{"80": {{HostIP: "::1"}}}),
		container.WithCmd("httpd", "-f"),
	)
	defer c.ContainerRemove(ctx, serverId, containertypes.RemoveOptions{Force: true})

	inspect := container.Inspect(ctx, t, c, serverId)
	hostPort := inspect.NetworkSettings.Ports["80/tcp"][0].HostPort

	resp, err := http.Get("http://[::1]:" + hostPort)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(resp.StatusCode, 404))
}

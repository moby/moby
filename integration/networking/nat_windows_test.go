package networking

import (
	"context"
	"net"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/network"
	"github.com/docker/docker/testutil"
	"github.com/docker/go-connections/nat"
)

// TestNatNetworkICC tries to ping container ctr1 from container ctr2 using its hostname.
// Checks DNS resolution, and whether containers can communicate with each other.
// Regression test for https://github.com/moby/moby/issues/47370
func TestNatNetworkICC(t *testing.T) {
	ctx := setupTest(t)
	c := testEnv.APIClient()

	testcases := []struct {
		name    string
		netName string
	}{
		{
			name:    "Default nat network",
			netName: "nat",
		},
		{
			name:    "User defined nat network",
			netName: "mynat",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)

			if tc.netName != "nat" {
				network.CreateNoError(ctx, t, c, tc.netName,
					network.WithDriver("nat"),
				)
				defer network.RemoveNoError(ctx, t, c, tc.netName)
			}

			const ctr1Name = "ctr1"
			id1 := container.Run(ctx, t, c,
				container.WithName(ctr1Name),
				container.WithNetworkMode(tc.netName),
			)
			defer c.ContainerRemove(ctx, id1, containertypes.RemoveOptions{Force: true})

			pingCmd := []string{"ping", "-n", "1", "-w", "3000", ctr1Name}

			const ctr2Name = "ctr2"
			attachCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			res := container.RunAttach(attachCtx, t, c,
				container.WithName(ctr2Name),
				container.WithCmd(pingCmd...),
				container.WithNetworkMode(tc.netName),
			)
			defer c.ContainerRemove(ctx, res.ContainerID, containertypes.RemoveOptions{Force: true})

			assert.Check(t, is.Equal(res.ExitCode, 0))
			assert.Check(t, is.Equal(res.Stderr.Len(), 0))
			assert.Check(t, is.Contains(res.Stdout.String(), "Sent = 1, Received = 1, Lost = 0"))
		})
	}
}

// Check that a container on one network can reach a service in a container on
// another network, via a mapped port on the host.
func TestPortMappedHairpinWindows(t *testing.T) {
	ctx := setupTest(t)
	c := testEnv.APIClient()

	// Find an address on the test host.
	conn, err := net.Dial("tcp4", "hub.docker.com:80")
	assert.NilError(t, err)
	hostAddr := conn.LocalAddr().(*net.TCPAddr).IP.String()
	conn.Close()

	const serverNetName = "servernet"
	network.CreateNoError(ctx, t, c, serverNetName, network.WithDriver("nat"))
	defer network.RemoveNoError(ctx, t, c, serverNetName)
	const clientNetName = "clientnet"
	network.CreateNoError(ctx, t, c, clientNetName, network.WithDriver("nat"))
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

func TestWindowsNoDisableIPv4(t *testing.T) {
	ctx := setupTest(t)
	c := testEnv.APIClient()

	_, err := network.Create(ctx, c, "ipv6only",
		network.WithDriver("nat"),
		network.WithIPv4(false),
	)
	// This error message should change to "IPv4 cannot be disabled on Windows"
	// when "--experimental" is no longer required to disable IPv4. But, there's
	// no way to start a second daemon with "--experimental" in Windows CI.
	assert.Check(t, is.ErrorContains(err,
		"IPv4 can only be disabled if experimental features are enabled"))
}

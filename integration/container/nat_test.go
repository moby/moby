package container // import "github.com/docker/docker/integration/container"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/go-connections/nat"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestNetworkNat(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "FIXME")
	skip.If(t, testEnv.IsRemoteDaemon)

	ctx := setupTest(t)

	const msg = "it works"
	const port = 8080
	startServerContainer(ctx, t, msg, port)

	endpoint := getExternalAddress(t)

	var conn net.Conn
	addr := net.JoinHostPort(endpoint.String(), strconv.Itoa(port))
	poll.WaitOn(t, func(t poll.LogT) poll.Result {
		var err error
		conn, err = net.Dial("tcp", addr)
		if err != nil {
			return poll.Continue("waiting for %s to be accessible: %v", addr, err)
		}
		return poll.Success()
	})
	defer func() {
		assert.Check(t, conn.Close())
	}()

	data, err := io.ReadAll(conn)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(msg, strings.TrimSpace(string(data))))
}

func TestNetworkLocalhostTCPNat(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)

	ctx := setupTest(t)

	const msg = "hi yall"
	const port = 8081
	startServerContainer(ctx, t, msg, port)

	var conn net.Conn
	addr := net.JoinHostPort("localhost", strconv.Itoa(port))
	poll.WaitOn(t, func(t poll.LogT) poll.Result {
		var err error
		conn, err = net.Dial("tcp", addr)
		if err != nil {
			return poll.Continue("waiting for %s to be accessible: %v", addr, err)
		}
		return poll.Success()
	})
	defer func() {
		assert.Check(t, conn.Close())
	}()

	data, err := io.ReadAll(conn)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(msg, strings.TrimSpace(string(data))))
}

func TestNetworkLoopbackNat(t *testing.T) {
	skip.If(t, testEnv.GitHubActions, "FIXME: https://github.com/moby/moby/issues/41561")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "FIXME")
	skip.If(t, testEnv.IsRemoteDaemon)

	ctx := setupTest(t)

	msg := "it works"
	serverContainerID := startServerContainer(ctx, t, msg, 8080)

	endpoint := getExternalAddress(t)

	apiClient := testEnv.APIClient()

	cID := container.Run(ctx, t, apiClient,
		container.WithCmd("sh", "-c", fmt.Sprintf("stty raw && nc -w 1 %s 8080", endpoint.String())),
		container.WithTty(true),
		container.WithNetworkMode("container:"+serverContainerID),
	)

	poll.WaitOn(t, container.IsStopped(ctx, apiClient, cID))

	body, err := apiClient.ContainerLogs(ctx, cID, containertypes.LogsOptions{
		ShowStdout: true,
	})
	assert.NilError(t, err)
	defer body.Close()

	var b bytes.Buffer
	_, err = io.Copy(&b, body)
	assert.NilError(t, err)

	assert.Check(t, is.Equal(msg, strings.TrimSpace(b.String())))
}

func startServerContainer(ctx context.Context, t *testing.T, msg string, port int) string {
	t.Helper()
	apiClient := testEnv.APIClient()

	return container.Run(ctx, t, apiClient,
		container.WithName("server-"+t.Name()),
		container.WithCmd("sh", "-c", fmt.Sprintf("echo %q | nc -lp %d", msg, port)),
		container.WithExposedPorts(fmt.Sprintf("%d/tcp", port)),
		func(c *container.TestContainerConfig) {
			c.HostConfig.PortBindings = nat.PortMap{
				nat.Port(fmt.Sprintf("%d/tcp", port)): []nat.PortBinding{
					{
						HostPort: fmt.Sprintf("%d", port),
					},
				},
			}
		},
	)
}

// getExternalAddress() returns the external IP-address from eth0. If eth0 has
// multiple IP-addresses, it returns the first IPv4 IP-address; if no IPv4
// address is present, it returns the first IP-address found.
func getExternalAddress(t *testing.T) net.IP {
	t.Helper()
	iface, err := net.InterfaceByName("eth0")
	skip.If(t, err != nil, "Test not running with `make test-integration`. Interface eth0 not found: %s", err)

	ifaceAddrs, err := iface.Addrs()
	assert.NilError(t, err)
	assert.Check(t, 0 != len(ifaceAddrs))

	if len(ifaceAddrs) > 1 {
		// Prefer IPv4 address if multiple addresses found, as rootlesskit
		// does not handle IPv6 currently https://github.com/moby/moby/pull/41908#issuecomment-774200001
		for _, a := range ifaceAddrs {
			ifaceIP, _, err := net.ParseCIDR(a.String())
			assert.NilError(t, err)
			if ifaceIP.To4() != nil {
				return ifaceIP
			}
		}
	}
	ifaceIP, _, err := net.ParseCIDR(ifaceAddrs[0].String())
	assert.NilError(t, err)

	return ifaceIP
}

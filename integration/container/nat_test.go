package container

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/integration/internal/request"
	"github.com/docker/go-connections/nat"
	"github.com/gotestyourself/gotestyourself/poll"
	"github.com/gotestyourself/gotestyourself/skip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNetworkNat(t *testing.T) {
	skip.If(t, !testEnv.IsLocalDaemon())

	defer setupTest(t)()

	msg := "it works"
	startServerContainer(t, msg, 8080)

	endpoint := getExternalAddress(t)
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", endpoint.String(), 8080))
	require.NoError(t, err)
	defer conn.Close()

	data, err := ioutil.ReadAll(conn)
	require.NoError(t, err)
	assert.Equal(t, strings.TrimSpace(string(data)), msg)
}

func TestNetworkLocalhostTCPNat(t *testing.T) {
	skip.If(t, !testEnv.IsLocalDaemon())

	defer setupTest(t)()

	msg := "hi yall"
	startServerContainer(t, msg, 8081)

	conn, err := net.Dial("tcp", "localhost:8081")
	require.NoError(t, err)
	defer conn.Close()

	data, err := ioutil.ReadAll(conn)
	require.NoError(t, err)
	assert.Equal(t, strings.TrimSpace(string(data)), msg)
}

func TestNetworkLoopbackNat(t *testing.T) {
	skip.If(t, !testEnv.IsLocalDaemon())

	msg := "it works"
	startServerContainer(t, msg, 8080)

	endpoint := getExternalAddress(t)

	client := request.NewAPIClient(t)
	ctx := context.Background()
	c, err := client.ContainerCreate(ctx,
		&container.Config{
			Image: "busybox",
			Cmd:   []string{"sh", "-c", fmt.Sprintf("stty raw && nc -w 5 %s 8080", endpoint.String())},
			Tty:   true,
		},
		&container.HostConfig{
			NetworkMode: "container:server",
		},
		nil,
		"")
	require.NoError(t, err)

	err = client.ContainerStart(ctx, c.ID, types.ContainerStartOptions{})
	require.NoError(t, err)

	poll.WaitOn(t, containerIsStopped(ctx, client, c.ID), poll.WithDelay(100*time.Millisecond))

	body, err := client.ContainerLogs(ctx, c.ID, types.ContainerLogsOptions{
		ShowStdout: true,
	})
	require.NoError(t, err)
	defer body.Close()

	var b bytes.Buffer
	_, err = io.Copy(&b, body)
	require.NoError(t, err)

	assert.Equal(t, strings.TrimSpace(b.String()), msg)
}

func startServerContainer(t *testing.T, msg string, port int) string {
	client := request.NewAPIClient(t)
	ctx := context.Background()

	c, err := client.ContainerCreate(ctx,
		&container.Config{
			Image: "busybox",
			Cmd:   []string{"sh", "-c", fmt.Sprintf("echo %q | nc -lp %d", msg, port)},
			ExposedPorts: map[nat.Port]struct{}{
				nat.Port(fmt.Sprintf("%d/tcp", port)): {},
			},
		},
		&container.HostConfig{
			PortBindings: nat.PortMap{
				nat.Port(fmt.Sprintf("%d/tcp", port)): []nat.PortBinding{
					{
						HostPort: fmt.Sprintf("%d", port),
					},
				},
			},
		},
		&network.NetworkingConfig{},
		"server",
	)
	require.NoError(t, err)

	err = client.ContainerStart(ctx, c.ID, types.ContainerStartOptions{})
	require.NoError(t, err)

	poll.WaitOn(t, containerIsInState(ctx, client, c.ID, "running"), poll.WithDelay(100*time.Millisecond))

	return c.ID
}

func getExternalAddress(t *testing.T) net.IP {
	iface, err := net.InterfaceByName("eth0")
	skip.If(t, err != nil, "Test not running with `make test-integration`. Interface eth0 not found: %s", err)

	ifaceAddrs, err := iface.Addrs()
	require.NoError(t, err)
	assert.NotEqual(t, len(ifaceAddrs), 0)

	ifaceIP, _, err := net.ParseCIDR(ifaceAddrs[0].String())
	require.NoError(t, err)

	return ifaceIP
}

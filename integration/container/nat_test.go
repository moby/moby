package container // import "github.com/docker/docker/integration/container"

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
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/internal/test/request"
	"github.com/docker/go-connections/nat"
	"github.com/gotestyourself/gotestyourself/assert"
	is "github.com/gotestyourself/gotestyourself/assert/cmp"
	"github.com/gotestyourself/gotestyourself/poll"
	"github.com/gotestyourself/gotestyourself/skip"
)

func TestNetworkNat(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon())

	defer setupTest(t)()

	msg := "it works"
	startServerContainer(t, msg, 8080)

	endpoint := getExternalAddress(t)
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", endpoint.String(), 8080))
	assert.NilError(t, err)
	defer conn.Close()

	data, err := ioutil.ReadAll(conn)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(msg, strings.TrimSpace(string(data))))
}

func TestNetworkLocalhostTCPNat(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon())

	defer setupTest(t)()

	msg := "hi yall"
	startServerContainer(t, msg, 8081)

	conn, err := net.Dial("tcp", "localhost:8081")
	assert.NilError(t, err)
	defer conn.Close()

	data, err := ioutil.ReadAll(conn)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(msg, strings.TrimSpace(string(data))))
}

func TestNetworkLoopbackNat(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon())

	defer setupTest(t)()

	msg := "it works"
	serverContainerID := startServerContainer(t, msg, 8080)

	endpoint := getExternalAddress(t)

	client := request.NewAPIClient(t)
	ctx := context.Background()

	cID := container.Run(t, ctx, client, container.WithCmd("sh", "-c", fmt.Sprintf("stty raw && nc -w 5 %s 8080", endpoint.String())), container.WithTty(true), container.WithNetworkMode("container:"+serverContainerID))

	poll.WaitOn(t, container.IsStopped(ctx, client, cID), poll.WithDelay(100*time.Millisecond))

	body, err := client.ContainerLogs(ctx, cID, types.ContainerLogsOptions{
		ShowStdout: true,
	})
	assert.NilError(t, err)
	defer body.Close()

	var b bytes.Buffer
	_, err = io.Copy(&b, body)
	assert.NilError(t, err)

	assert.Check(t, is.Equal(msg, strings.TrimSpace(b.String())))
}

func startServerContainer(t *testing.T, msg string, port int) string {
	client := request.NewAPIClient(t)
	ctx := context.Background()

	cID := container.Run(t, ctx, client, container.WithName("server-"+t.Name()), container.WithCmd("sh", "-c", fmt.Sprintf("echo %q | nc -lp %d", msg, port)), container.WithExposedPorts(fmt.Sprintf("%d/tcp", port)), func(c *container.TestContainerConfig) {
		c.HostConfig.PortBindings = nat.PortMap{
			nat.Port(fmt.Sprintf("%d/tcp", port)): []nat.PortBinding{
				{
					HostPort: fmt.Sprintf("%d", port),
				},
			},
		}
	})

	poll.WaitOn(t, container.IsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))

	return cID
}

func getExternalAddress(t *testing.T) net.IP {
	iface, err := net.InterfaceByName("eth0")
	skip.If(t, err != nil, "Test not running with `make test-integration`. Interface eth0 not found: %s", err)

	ifaceAddrs, err := iface.Addrs()
	assert.NilError(t, err)
	assert.Check(t, 0 != len(ifaceAddrs))

	ifaceIP, _, err := net.ParseCIDR(ifaceAddrs[0].String())
	assert.NilError(t, err)

	return ifaceIP
}

package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	containerTypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/runconfig"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestUtsHost(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, testEnv.IsRemoteDaemon())

	hostUts, err := os.Readlink("/proc/1/ns/uts")
	assert.NilError(t, err)

	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	cID := container.Run(ctx, t, client, func(c *container.TestContainerConfig) {
		c.HostConfig.UTSMode = "host"
	})
	poll.WaitOn(t, container.IsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))
	cUts := container.GetContainerNS(ctx, t, client, cID, "uts")
	assert.Assert(t, hostUts == cUts)

	cID = container.Run(ctx, t, client)
	poll.WaitOn(t, container.IsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))
	cUts = container.GetContainerNS(ctx, t, client, cID, "uts")
	assert.Assert(t, hostUts != cUts)
}

func TestUtsContainerNotExists(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, testEnv.IsRemoteDaemon())

	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	cID := container.Create(ctx, t, client, func(c *container.TestContainerConfig) {
		c.HostConfig.UTSMode = "container:non_existent"
	})

	err := client.ContainerStart(ctx, cID, types.ContainerStartOptions{})
	assert.Check(t, is.ErrorContains(err, "non_existent"))
}

func TestUtsContainerNotRunning(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, testEnv.IsRemoteDaemon())

	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	// create parent container
	parentID := container.Create(ctx, t, client)

	cID := container.Create(ctx, t, client, func(c *container.TestContainerConfig) {
		c.HostConfig.UTSMode = containerTypes.UTSMode("container:" + parentID)
	})
	err := client.ContainerStart(ctx, cID, types.ContainerStartOptions{})
	assert.Check(t, is.ErrorContains(err, "is not running"))
}

func TestUtsContainer(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, testEnv.IsRemoteDaemon())

	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	parentID := container.Run(ctx, t, client)
	poll.WaitOn(t, container.IsInState(ctx, client, parentID, "running"), poll.WithDelay(100*time.Millisecond))
	parentUts := container.GetContainerNS(ctx, t, client, parentID, "uts")

	cID := container.Run(ctx, t, client, func(c *container.TestContainerConfig) {
		c.HostConfig.UTSMode = containerTypes.UTSMode("container:" + parentID)
	})
	poll.WaitOn(t, container.IsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))
	cUts := container.GetContainerNS(ctx, t, client, cID, "uts")
	assert.Assert(t, parentUts == cUts)
}

func TestUtsArgumentsConflict(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, testEnv.IsRemoteDaemon())

	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	container.CreateExpectingErr(ctx, t, client, runconfig.ErrConflictUTSHostname.Error(), func(c *container.TestContainerConfig) {
		c.Config.Hostname = "abcd1234"
		c.HostConfig.UTSMode = "container:efgh5678"
	})

	container.CreateExpectingErr(ctx, t, client, runconfig.ErrConflictUTSDomainname.Error(), func(c *container.TestContainerConfig) {
		c.Config.Domainname = "abcd1234"
		c.HostConfig.UTSMode = "container:efgh5678"
	})

	container.CreateExpectingErr(ctx, t, client, runconfig.ErrConflictUTSHostname.Error(), func(c *container.TestContainerConfig) {
		c.Config.Hostname = "abcd1234"
		c.HostConfig.UTSMode = "host"
	})

	container.CreateExpectingErr(ctx, t, client, runconfig.ErrConflictUTSDomainname.Error(), func(c *container.TestContainerConfig) {
		c.Config.Domainname = "abcd1234"
		c.HostConfig.UTSMode = "host"
	})
}

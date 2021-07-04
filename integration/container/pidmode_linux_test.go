package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/integration/internal/container"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestPidHost(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, testEnv.IsRemoteDaemon())

	hostPid, err := os.Readlink("/proc/1/ns/pid")
	assert.NilError(t, err)

	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	cID := container.Run(ctx, t, client, func(c *container.TestContainerConfig) {
		c.HostConfig.PidMode = "host"
	})
	poll.WaitOn(t, container.IsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))
	cPid := container.GetContainerNS(ctx, t, client, cID, "pid")
	assert.Assert(t, hostPid == cPid)

	cID = container.Run(ctx, t, client)
	poll.WaitOn(t, container.IsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))
	cPid = container.GetContainerNS(ctx, t, client, cID, "pid")
	assert.Assert(t, hostPid != cPid)
}

package container // import "github.com/docker/docker/integration/container"

import (
	"os"
	"testing"
	"time"

	"github.com/docker/docker/integration/internal/container"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestPIDModeHost(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, testEnv.IsRemoteDaemon())

	hostPid, err := os.Readlink("/proc/1/ns/pid")
	assert.NilError(t, err)

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	cID := container.Run(ctx, t, apiClient, container.WithPIDMode("host"))
	poll.WaitOn(t, container.IsInState(ctx, apiClient, cID, "running"), poll.WithDelay(100*time.Millisecond))
	cPid := container.GetContainerNS(ctx, t, apiClient, cID, "pid")
	assert.Assert(t, hostPid == cPid)

	cID = container.Run(ctx, t, apiClient)
	poll.WaitOn(t, container.IsInState(ctx, apiClient, cID, "running"), poll.WithDelay(100*time.Millisecond))
	cPid = container.GetContainerNS(ctx, t, apiClient, cID, "pid")
	assert.Assert(t, hostPid != cPid)
}

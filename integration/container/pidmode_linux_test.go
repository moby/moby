package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/integration/internal/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestPIDModeHost(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, testEnv.IsRemoteDaemon())

	hostPid, err := os.Readlink("/proc/1/ns/pid")
	assert.NilError(t, err)

	defer setupTest(t)()
	apiClient := testEnv.APIClient()
	ctx := context.Background()

	cID := container.Run(ctx, t, apiClient, container.WithPIDMode("host"))
	poll.WaitOn(t, container.IsInState(ctx, apiClient, cID, "running"), poll.WithDelay(100*time.Millisecond))
	cPid := container.GetContainerNS(ctx, t, apiClient, cID, "pid")
	assert.Assert(t, hostPid == cPid)

	cID = container.Run(ctx, t, apiClient)
	poll.WaitOn(t, container.IsInState(ctx, apiClient, cID, "running"), poll.WithDelay(100*time.Millisecond))
	cPid = container.GetContainerNS(ctx, t, apiClient, cID, "pid")
	assert.Assert(t, hostPid != cPid)
}

func TestPIDModeContainer(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	defer setupTest(t)()
	apiClient := testEnv.APIClient()
	ctx := context.Background()

	t.Run("non-existing container", func(t *testing.T) {
		_, err := container.CreateFromConfig(ctx, apiClient, container.NewTestConfig(container.WithPIDMode("container:nosuchcontainer")))
		assert.Check(t, is.ErrorType(err, errdefs.IsInvalidParameter))
		assert.Check(t, is.ErrorContains(err, "No such container: nosuchcontainer"))
	})

	t.Run("non-running container", func(t *testing.T) {
		const pidCtrName = "stopped-pid-namespace-container"
		cPIDContainerID := container.Create(ctx, t, apiClient, container.WithName(pidCtrName))

		ctr, err := container.CreateFromConfig(ctx, apiClient, container.NewTestConfig(container.WithPIDMode("container:"+pidCtrName)))
		assert.NilError(t, err, "should not produce an error when creating, only when starting")

		err = apiClient.ContainerStart(ctx, ctr.ID, types.ContainerStartOptions{})
		assert.Check(t, is.ErrorType(err, errdefs.IsSystem), "should produce a System error when starting an existing container from an invalid state")
		assert.Check(t, is.ErrorContains(err, "failed to join PID namespace"))
		assert.Check(t, is.ErrorContains(err, cPIDContainerID+" is not running"))
	})

	t.Run("running container", func(t *testing.T) {
		const pidCtrName = "running-pid-namespace-container"
		container.Run(ctx, t, apiClient, container.WithName(pidCtrName))

		ctr, err := container.CreateFromConfig(ctx, apiClient, container.NewTestConfig(container.WithPIDMode("container:"+pidCtrName)))
		assert.NilError(t, err)

		err = apiClient.ContainerStart(ctx, ctr.ID, types.ContainerStartOptions{})
		assert.Check(t, err)
	})
}

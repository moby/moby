package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/internal/test/request"
	"github.com/gotestyourself/gotestyourself/assert"
	is "github.com/gotestyourself/gotestyourself/assert/cmp"
	"github.com/gotestyourself/gotestyourself/poll"
	"github.com/gotestyourself/gotestyourself/skip"
)

func TestKillContainerInvalidSignal(t *testing.T) {
	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()
	id := container.Run(t, ctx, client)

	err := client.ContainerKill(ctx, id, "0")
	assert.Error(t, err, "Error response from daemon: Invalid signal: 0")
	poll.WaitOn(t, container.IsInState(ctx, client, id, "running"), poll.WithDelay(100*time.Millisecond))

	err = client.ContainerKill(ctx, id, "SIG42")
	assert.Error(t, err, "Error response from daemon: Invalid signal: SIG42")
	poll.WaitOn(t, container.IsInState(ctx, client, id, "running"), poll.WithDelay(100*time.Millisecond))
}

func TestKillContainer(t *testing.T) {
	defer setupTest(t)()
	client := request.NewAPIClient(t)

	testCases := []struct {
		doc    string
		signal string
		status string
	}{
		{
			doc:    "no signal",
			signal: "",
			status: "exited",
		},
		{
			doc:    "non killing signal",
			signal: "SIGWINCH",
			status: "running",
		},
		{
			doc:    "killing signal",
			signal: "SIGTERM",
			status: "exited",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.doc, func(t *testing.T) {
			ctx := context.Background()
			id := container.Run(t, ctx, client)
			err := client.ContainerKill(ctx, id, tc.signal)
			assert.NilError(t, err)

			poll.WaitOn(t, container.IsInState(ctx, client, id, tc.status), poll.WithDelay(100*time.Millisecond))
		})
	}
}

func TestKillWithStopSignalAndRestartPolicies(t *testing.T) {
	skip.If(t, testEnv.OSType != "linux", "Windows only supports 1.25 or later")
	defer setupTest(t)()
	client := request.NewAPIClient(t)

	testCases := []struct {
		doc        string
		stopsignal string
		status     string
	}{
		{
			doc:        "same-signal-disables-restart-policy",
			stopsignal: "TERM",
			status:     "exited",
		},
		{
			doc:        "different-signal-keep-restart-policy",
			stopsignal: "CONT",
			status:     "running",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.doc, func(t *testing.T) {
			ctx := context.Background()
			id := container.Run(t, ctx, client, func(c *container.TestContainerConfig) {
				c.Config.StopSignal = tc.stopsignal
				c.HostConfig.RestartPolicy = containertypes.RestartPolicy{
					Name: "always",
				}
			})
			err := client.ContainerKill(ctx, id, "TERM")
			assert.NilError(t, err)

			poll.WaitOn(t, container.IsInState(ctx, client, id, tc.status), poll.WithDelay(100*time.Millisecond))
		})
	}
}

func TestKillStoppedContainer(t *testing.T) {
	skip.If(t, testEnv.OSType != "linux") // Windows only supports 1.25 or later
	defer setupTest(t)()
	ctx := context.Background()
	client := request.NewAPIClient(t)
	id := container.Create(t, ctx, client)
	err := client.ContainerKill(ctx, id, "SIGKILL")
	assert.Assert(t, is.ErrorContains(err, ""))
	assert.Assert(t, is.Contains(err.Error(), "is not running"))
}

func TestKillStoppedContainerAPIPre120(t *testing.T) {
	skip.If(t, testEnv.OSType != "linux") // Windows only supports 1.25 or later
	defer setupTest(t)()
	ctx := context.Background()
	client := request.NewAPIClient(t, client.WithVersion("1.19"))
	id := container.Create(t, ctx, client)
	err := client.ContainerKill(ctx, id, "SIGKILL")
	assert.NilError(t, err)
}

func TestKillDifferentUserContainer(t *testing.T) {
	// TODO Windows: Windows does not yet support -u (Feb 2016).
	skip.If(t, testEnv.OSType != "linux", "User containers (container.Config.User) are not yet supported on %q platform", testEnv.OSType)

	defer setupTest(t)()
	ctx := context.Background()
	client := request.NewAPIClient(t, client.WithVersion("1.19"))

	id := container.Run(t, ctx, client, func(c *container.TestContainerConfig) {
		c.Config.User = "daemon"
	})
	poll.WaitOn(t, container.IsInState(ctx, client, id, "running"), poll.WithDelay(100*time.Millisecond))

	err := client.ContainerKill(ctx, id, "SIGKILL")
	assert.NilError(t, err)
	poll.WaitOn(t, container.IsInState(ctx, client, id, "exited"), poll.WithDelay(100*time.Millisecond))
}

func TestInspectOomKilledTrue(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux" || !testEnv.DaemonInfo.MemoryLimit || !testEnv.DaemonInfo.SwapLimit)

	defer setupTest(t)()
	ctx := context.Background()
	client := request.NewAPIClient(t)

	cID := container.Run(t, ctx, client, container.WithCmd("sh", "-c", "x=a; while true; do x=$x$x$x$x; done"), func(c *container.TestContainerConfig) {
		c.HostConfig.Resources.Memory = 32 * 1024 * 1024
	})

	poll.WaitOn(t, container.IsInState(ctx, client, cID, "exited"), poll.WithDelay(100*time.Millisecond))

	inspect, err := client.ContainerInspect(ctx, cID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(true, inspect.State.OOMKilled))
}

func TestInspectOomKilledFalse(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux" || !testEnv.DaemonInfo.MemoryLimit || !testEnv.DaemonInfo.SwapLimit)

	defer setupTest(t)()
	ctx := context.Background()
	client := request.NewAPIClient(t)

	cID := container.Run(t, ctx, client, container.WithCmd("sh", "-c", "echo hello world"))

	poll.WaitOn(t, container.IsInState(ctx, client, cID, "exited"), poll.WithDelay(100*time.Millisecond))

	inspect, err := client.ContainerInspect(ctx, cID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(false, inspect.State.OOMKilled))
}

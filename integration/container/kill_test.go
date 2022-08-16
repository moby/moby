package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestKillContainerInvalidSignal(t *testing.T) {
	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()
	id := container.Run(ctx, t, client)

	err := client.ContainerKill(ctx, id, "0")
	assert.ErrorContains(t, err, "Error response from daemon:")
	assert.ErrorContains(t, err, "nvalid signal: 0") // match "(I|i)nvalid" case-insensitive to allow testing against older daemons.
	poll.WaitOn(t, container.IsInState(ctx, client, id, "running"), poll.WithDelay(100*time.Millisecond))

	err = client.ContainerKill(ctx, id, "SIG42")
	assert.ErrorContains(t, err, "Error response from daemon:")
	assert.ErrorContains(t, err, "nvalid signal: SIG42") // match "(I|i)nvalid" case-insensitive to allow testing against older daemons.
	poll.WaitOn(t, container.IsInState(ctx, client, id, "running"), poll.WithDelay(100*time.Millisecond))
}

func TestKillContainer(t *testing.T) {
	defer setupTest(t)()
	client := testEnv.APIClient()

	testCases := []struct {
		doc    string
		signal string
		status string
		skipOs string
	}{
		{
			doc:    "no signal",
			signal: "",
			status: "exited",
			skipOs: "",
		},
		{
			doc:    "non killing signal",
			signal: "SIGWINCH",
			status: "running",
			skipOs: "windows",
		},
		{
			doc:    "killing signal",
			signal: "SIGTERM",
			status: "exited",
			skipOs: "",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.doc, func(t *testing.T) {
			skip.If(t, testEnv.OSType == tc.skipOs, "Windows does not support SIGWINCH")
			ctx := context.Background()
			id := container.Run(ctx, t, client)
			err := client.ContainerKill(ctx, id, tc.signal)
			assert.NilError(t, err)

			poll.WaitOn(t, container.IsInState(ctx, client, id, tc.status), poll.WithDelay(100*time.Millisecond))
		})
	}
}

func TestKillWithStopSignalAndRestartPolicies(t *testing.T) {
	skip.If(t, testEnv.OSType == "windows", "Windows only supports 1.25 or later")
	defer setupTest(t)()
	client := testEnv.APIClient()

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
			id := container.Run(ctx, t, client,
				container.WithRestartPolicy("always"),
				func(c *container.TestContainerConfig) {
					c.Config.StopSignal = tc.stopsignal
				})
			err := client.ContainerKill(ctx, id, "TERM")
			assert.NilError(t, err)

			poll.WaitOn(t, container.IsInState(ctx, client, id, tc.status), poll.WithDelay(100*time.Millisecond))
		})
	}
}

func TestKillStoppedContainer(t *testing.T) {
	skip.If(t, testEnv.OSType == "windows", "Windows only supports 1.25 or later")
	defer setupTest(t)()
	ctx := context.Background()
	client := testEnv.APIClient()
	id := container.Create(ctx, t, client)
	err := client.ContainerKill(ctx, id, "SIGKILL")
	assert.Assert(t, is.ErrorContains(err, ""))
	assert.Assert(t, is.Contains(err.Error(), "is not running"))
}

func TestKillStoppedContainerAPIPre120(t *testing.T) {
	skip.If(t, testEnv.OSType == "windows", "Windows only supports 1.25 or later")
	defer setupTest(t)()
	ctx := context.Background()
	client := request.NewAPIClient(t, client.WithVersion("1.19"))
	id := container.Create(ctx, t, client)
	err := client.ContainerKill(ctx, id, "SIGKILL")
	assert.NilError(t, err)
}

func TestKillDifferentUserContainer(t *testing.T) {
	// TODO Windows: Windows does not yet support -u (Feb 2016).
	skip.If(t, testEnv.OSType == "windows", "User containers (container.Config.User) are not yet supported on %q platform", testEnv.OSType)

	defer setupTest(t)()
	ctx := context.Background()
	client := request.NewAPIClient(t, client.WithVersion("1.19"))

	id := container.Run(ctx, t, client, func(c *container.TestContainerConfig) {
		c.Config.User = "daemon"
	})
	poll.WaitOn(t, container.IsInState(ctx, client, id, "running"), poll.WithDelay(100*time.Millisecond))

	err := client.ContainerKill(ctx, id, "SIGKILL")
	assert.NilError(t, err)
	poll.WaitOn(t, container.IsInState(ctx, client, id, "exited"), poll.WithDelay(100*time.Millisecond))
}

func TestInspectOomKilledTrue(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, testEnv.DaemonInfo.CgroupDriver == "none")
	skip.If(t, !testEnv.DaemonInfo.MemoryLimit || !testEnv.DaemonInfo.SwapLimit)
	skip.If(t, testEnv.DaemonInfo.CgroupVersion == "2", "FIXME: flaky on cgroup v2 (https://github.com/moby/moby/issues/41929)")

	defer setupTest(t)()
	ctx := context.Background()
	client := testEnv.APIClient()

	cID := container.Run(ctx, t, client, container.WithCmd("sh", "-c", "x=a; while true; do x=$x$x$x$x; done"), func(c *container.TestContainerConfig) {
		c.HostConfig.Resources.Memory = 32 * 1024 * 1024
	})

	poll.WaitOn(t, container.IsInState(ctx, client, cID, "exited"), poll.WithDelay(100*time.Millisecond))

	inspect, err := client.ContainerInspect(ctx, cID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(true, inspect.State.OOMKilled))
}

func TestInspectOomKilledFalse(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows" || !testEnv.DaemonInfo.MemoryLimit || !testEnv.DaemonInfo.SwapLimit)

	defer setupTest(t)()
	ctx := context.Background()
	client := testEnv.APIClient()

	cID := container.Run(ctx, t, client, container.WithCmd("sh", "-c", "echo hello world"))

	poll.WaitOn(t, container.IsInState(ctx, client, cID, "exited"), poll.WithDelay(100*time.Millisecond))

	inspect, err := client.ContainerInspect(ctx, cID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(false, inspect.State.OOMKilled))
}

package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"io"
	"testing"
	"time"

	containerderrdefs "github.com/containerd/containerd/errdefs"
	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestPause(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows" && testEnv.DaemonInfo.Isolation == "process")
	skip.If(t, testEnv.DaemonInfo.CgroupDriver == "none")

	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	cID := container.Run(ctx, t, client)
	poll.WaitOn(t, container.IsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))

	since := request.DaemonUnixTime(ctx, t, client, testEnv)

	err := client.ContainerPause(ctx, cID)
	assert.NilError(t, err)

	inspect, err := client.ContainerInspect(ctx, cID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(true, inspect.State.Paused))

	err = client.ContainerUnpause(ctx, cID)
	assert.NilError(t, err)

	until := request.DaemonUnixTime(ctx, t, client, testEnv)

	messages, errs := client.Events(ctx, types.EventsOptions{
		Since:   since,
		Until:   until,
		Filters: filters.NewArgs(filters.Arg("container", cID)),
	})
	assert.Check(t, is.DeepEqual([]string{"pause", "unpause"}, getEventActions(t, messages, errs)))
}

func TestPauseFailsOnWindowsServerContainers(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "windows" || testEnv.DaemonInfo.Isolation != "process")

	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	cID := container.Run(ctx, t, client)
	poll.WaitOn(t, container.IsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))

	err := client.ContainerPause(ctx, cID)
	assert.Check(t, is.ErrorContains(err, containerderrdefs.ErrNotImplemented.Error()))
}

func TestPauseStopPausedContainer(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.31"), "broken in earlier versions")
	skip.If(t, testEnv.DaemonInfo.CgroupDriver == "none")
	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	cID := container.Run(ctx, t, client)
	poll.WaitOn(t, container.IsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))

	err := client.ContainerPause(ctx, cID)
	assert.NilError(t, err)

	err = client.ContainerStop(ctx, cID, containertypes.StopOptions{})
	assert.NilError(t, err)

	poll.WaitOn(t, container.IsStopped(ctx, client, cID), poll.WithDelay(100*time.Millisecond))
}

func getEventActions(t *testing.T, messages <-chan events.Message, errs <-chan error) []string {
	var actions []string
	for {
		select {
		case err := <-errs:
			assert.Check(t, err == nil || err == io.EOF)
			return actions
		case e := <-messages:
			actions = append(actions, e.Status)
		}
	}
}

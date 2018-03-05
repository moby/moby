package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/request"
	"github.com/docker/docker/internal/testutil"
	"github.com/gotestyourself/gotestyourself/poll"
	"github.com/gotestyourself/gotestyourself/skip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPause(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows" && testEnv.DaemonInfo.Isolation == "process")

	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	cID := container.Run(t, ctx, client)
	poll.WaitOn(t, container.IsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))

	since := request.DaemonUnixTime(ctx, t, client, testEnv)

	err := client.ContainerPause(ctx, cID)
	require.NoError(t, err)

	inspect, err := client.ContainerInspect(ctx, cID)
	require.NoError(t, err)
	assert.Equal(t, true, inspect.State.Paused)

	err = client.ContainerUnpause(ctx, cID)
	require.NoError(t, err)

	until := request.DaemonUnixTime(ctx, t, client, testEnv)

	messages, errs := client.Events(ctx, types.EventsOptions{
		Since:   since,
		Until:   until,
		Filters: filters.NewArgs(filters.Arg("container", cID)),
	})
	assert.Equal(t, []string{"pause", "unpause"}, getEventActions(t, messages, errs))
}

func TestPauseFailsOnWindowsServerContainers(t *testing.T) {
	skip.If(t, (testEnv.DaemonInfo.OSType != "windows" || testEnv.DaemonInfo.Isolation != "process"))

	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	cID := container.Run(t, ctx, client)
	poll.WaitOn(t, container.IsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))

	err := client.ContainerPause(ctx, cID)
	testutil.ErrorContains(t, err, "cannot pause Windows Server Containers")
}

func TestPauseStopPausedContainer(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	cID := container.Run(t, ctx, client)
	poll.WaitOn(t, container.IsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))

	err := client.ContainerPause(ctx, cID)
	require.NoError(t, err)

	err = client.ContainerStop(ctx, cID, nil)
	require.NoError(t, err)

	poll.WaitOn(t, container.IsStopped(ctx, client, cID), poll.WithDelay(100*time.Millisecond))
}

func getEventActions(t *testing.T, messages <-chan events.Message, errs <-chan error) []string {
	actions := []string{}
	for {
		select {
		case err := <-errs:
			assert.True(t, err == nil || err == io.EOF)
			return actions
		case e := <-messages:
			actions = append(actions, e.Status)
		}
	}
}

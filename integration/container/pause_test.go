package container

import (
	"errors"
	"io"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/integration/internal/container"
	"github.com/moby/moby/v2/internal/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestPause(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows" && testEnv.DaemonInfo.Isolation == "process")
	skip.If(t, testEnv.DaemonInfo.CgroupDriver == "none")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	cID := container.Run(ctx, t, apiClient)

	since := request.DaemonUnixTime(ctx, t, apiClient, testEnv)

	err := apiClient.ContainerPause(ctx, cID)
	assert.NilError(t, err)

	inspect, err := apiClient.ContainerInspect(ctx, cID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(true, inspect.State.Paused))

	err = apiClient.ContainerUnpause(ctx, cID)
	assert.NilError(t, err)

	until := request.DaemonUnixTime(ctx, t, apiClient, testEnv)

	messages, errs := apiClient.Events(ctx, client.EventsListOptions{
		Since:   since,
		Until:   until,
		Filters: make(client.Filters).Add(string(events.ContainerEventType), cID),
	})
	assert.Check(t, is.DeepEqual([]events.Action{events.ActionPause, events.ActionUnPause}, getEventActions(t, messages, errs)))
}

func TestPauseFailsOnWindowsServerContainers(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "windows" || testEnv.DaemonInfo.Isolation != "process")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	cID := container.Run(ctx, t, apiClient)
	err := apiClient.ContainerPause(ctx, cID)
	assert.Check(t, is.ErrorContains(err, cerrdefs.ErrNotImplemented.Error()))
}

func TestPauseStopPausedContainer(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, testEnv.DaemonInfo.CgroupDriver == "none")
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	cID := container.Run(ctx, t, apiClient)
	err := apiClient.ContainerPause(ctx, cID)
	assert.NilError(t, err)

	err = apiClient.ContainerStop(ctx, cID, client.ContainerStopOptions{})
	assert.NilError(t, err)

	poll.WaitOn(t, container.IsStopped(ctx, apiClient, cID))
}

func getEventActions(t *testing.T, messages <-chan events.Message, errs <-chan error) []events.Action {
	t.Helper()
	var actions []events.Action
	for {
		select {
		case err := <-errs:
			assert.Check(t, err == nil || errors.Is(err, io.EOF))
			return actions
		case e := <-messages:
			actions = append(actions, e.Action)
		}
	}
}

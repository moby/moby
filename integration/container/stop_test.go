package container

import (
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

// hcs can sometimes take a long time to stop container.
const StopContainerWindowsPollTimeout = 75 * time.Second

func TestStopContainerWithRestartPolicyAlways(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	names := []string{"verifyRestart1-" + t.Name(), "verifyRestart2-" + t.Name()}
	for _, name := range names {
		container.Run(ctx, t, apiClient,
			container.WithName(name),
			container.WithCmd("false"),
			container.WithRestartPolicy(containertypes.RestartPolicyAlways),
		)
	}

	for _, name := range names {
		poll.WaitOn(t, container.IsInState(ctx, apiClient, name, containertypes.StateRunning, containertypes.StateRestarting))
	}

	for _, name := range names {
		err := apiClient.ContainerStop(ctx, name, containertypes.StopOptions{})
		assert.NilError(t, err)
	}

	for _, name := range names {
		poll.WaitOn(t, container.IsStopped(ctx, apiClient, name))
	}
}

// TestStopContainerWithTimeout checks that ContainerStop with
// a timeout works as documented, i.e. in case of negative timeout
// waiting is not limited (issue #35311).
func TestStopContainerWithTimeout(t *testing.T) {
	isWindows := testEnv.DaemonInfo.OSType == "windows"
	// TODO(vvoland): Make this work on Windows
	skip.If(t, isWindows)

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	forcefulKillExitCode := 137
	if isWindows {
		forcefulKillExitCode = 0x40010004
	}

	testCmd := container.WithCmd("sh", "-c", "sleep 10 && exit 42")
	testData := []struct {
		doc              string
		timeout          int
		expectedExitCode int
	}{
		// In case container is forcefully killed, 137 is returned,
		// otherwise the exit code from the above script
		{
			doc:              "zero timeout: expect forceful container kill",
			expectedExitCode: forcefulKillExitCode,
			timeout:          0,
		},
		{
			doc:              "too small timeout: expect forceful container kill",
			expectedExitCode: forcefulKillExitCode,
			timeout:          2,
		},
		{
			doc:              "big enough timeout: expect graceful container stop",
			expectedExitCode: 42,
			timeout:          20, // longer than "sleep 10" cmd
		},
		{
			doc:              "unlimited timeout: expect graceful container stop",
			expectedExitCode: 42,
			timeout:          -1,
		},
	}

	var pollOpts []poll.SettingOp
	if isWindows {
		pollOpts = append(pollOpts, poll.WithTimeout(StopContainerWindowsPollTimeout))
	}

	for _, tc := range testData {
		t.Run(tc.doc, func(t *testing.T) {
			// TODO(vvoland): Investigate why it helps
			// t.Parallel()
			id := container.Run(ctx, t, apiClient, testCmd)

			err := apiClient.ContainerStop(ctx, id, containertypes.StopOptions{Timeout: &tc.timeout})
			assert.NilError(t, err)

			poll.WaitOn(t, container.IsStopped(ctx, apiClient, id), pollOpts...)

			inspect, err := apiClient.ContainerInspect(ctx, id)
			assert.NilError(t, err)
			assert.Check(t, is.Equal(inspect.State.ExitCode, tc.expectedExitCode))
		})
	}
}

// TestStopEventsOrder verifies that "stop" and "die" events are produced in the
// correct order when stopping a container; see
// https://github.com/moby/moby/pull/50136#discussion_r2152578561
// https://github.com/docker/compose/pull/12950#issuecomment-2980759296
func TestStopEventsOrder(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	cID := container.Run(ctx, t, apiClient)

	since := request.DaemonUnixTime(ctx, t, apiClient, testEnv)

	err := apiClient.ContainerStop(ctx, cID, containertypes.StopOptions{})
	assert.NilError(t, err)

	poll.WaitOn(t, container.IsStopped(ctx, apiClient, cID))

	until := request.DaemonUnixTime(ctx, t, apiClient, testEnv)

	messages, errs := apiClient.Events(ctx, events.ListOptions{
		Since: since,
		Until: until,
		Filters: filters.NewArgs(
			filters.Arg("container", cID),

			// Only consider the event types we want to compare; prevent any
			// stray "start" event from being included, and don't look for
			// kill events, because a forced shutdown produces 2 instead of 1
			// kill event.
			filters.Arg("event", string(events.ActionStop)),
			filters.Arg("event", string(events.ActionDie)),
		),
	})
	assert.Check(t, is.DeepEqual([]events.Action{events.ActionStop, events.ActionDie}, getEventActions(t, messages, errs)))
}

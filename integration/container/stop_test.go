package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration/internal/container"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"
)

// hcs can sometimes take a long time to stop container.
const StopContainerWindowsPollTimeout = 75 * time.Second

func TestStopContainerWithRestartPolicyAlways(t *testing.T) {
	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	names := []string{"verifyRestart1-" + t.Name(), "verifyRestart2-" + t.Name()}
	for _, name := range names {
		container.Run(ctx, t, client,
			container.WithName(name),
			container.WithCmd("false"),
			container.WithRestartPolicy("always"),
		)
	}

	for _, name := range names {
		poll.WaitOn(t, container.IsInState(ctx, client, name, "running", "restarting"), poll.WithDelay(100*time.Millisecond))
	}

	for _, name := range names {
		err := client.ContainerStop(ctx, name, containertypes.StopOptions{})
		assert.NilError(t, err)
	}

	for _, name := range names {
		poll.WaitOn(t, container.IsStopped(ctx, client, name), poll.WithDelay(100*time.Millisecond))
	}
}

// TestStopContainerWithTimeout checks that ContainerStop with
// a timeout works as documented, i.e. in case of negative timeout
// waiting is not limited (issue #35311).
func TestStopContainerWithTimeout(t *testing.T) {
	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	forcefulKillExitCode := 137
	if testEnv.OSType == "windows" {
		forcefulKillExitCode = 0x40010004
	}

	testCmd := container.WithCmd("sleep", "10")
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
			expectedExitCode: 0,
			timeout:          20, // longer than "sleep 10" cmd
		},
		{
			doc:              "unlimited timeout: expect graceful container stop",
			expectedExitCode: 0,
			timeout:          -1,
		},
	}

	var pollOpts []poll.SettingOp
	if testEnv.OSType == "windows" {
		pollOpts = append(pollOpts, poll.WithTimeout(StopContainerWindowsPollTimeout))
	}

	for _, tc := range testData {
		tc := tc
		t.Run(tc.doc, func(t *testing.T) {
			t.Parallel()
			id := container.Run(ctx, t, client, testCmd)

			err := client.ContainerStop(ctx, id, containertypes.StopOptions{Timeout: &tc.timeout})
			assert.NilError(t, err)

			poll.WaitOn(t, container.IsStopped(ctx, client, id), pollOpts...)

			inspect, err := client.ContainerInspect(ctx, id)
			assert.NilError(t, err)
			assert.Equal(t, inspect.State.ExitCode, tc.expectedExitCode)
		})
	}
}

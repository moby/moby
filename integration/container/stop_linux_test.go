package container // import "github.com/docker/docker/integration/container"

import (
	"bytes"
	"context"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/pkg/stdcopy"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
)

// TestStopContainerWithTimeout checks that ContainerStop with
// a timeout works as documented, i.e. in case of negative timeout
// waiting is not limited (issue #35311).
func TestStopContainerWithTimeout(t *testing.T) {
	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	testCmd := container.WithCmd("sh", "-c", "sleep 2 && exit 42")
	testData := []struct {
		doc              string
		timeout          int
		expectedExitCode int
	}{
		// In case container is forcefully killed, 137 is returned,
		// otherwise the exit code from the above script
		{
			"zero timeout: expect forceful container kill",
			0, 137,
		},
		{
			"too small timeout: expect forceful container kill",
			1, 137,
		},
		{
			"big enough timeout: expect graceful container stop",
			3, 42,
		},
		{
			"unlimited timeout: expect graceful container stop",
			-1, 42,
		},
	}

	for _, d := range testData {
		d := d
		t.Run(strconv.Itoa(d.timeout), func(t *testing.T) {
			t.Parallel()
			id := container.Run(ctx, t, client, testCmd)

			err := client.ContainerStop(ctx, id, containertypes.StopOptions{Timeout: &d.timeout})
			assert.NilError(t, err)

			poll.WaitOn(t, container.IsStopped(ctx, client, id),
				poll.WithDelay(100*time.Millisecond))

			inspect, err := client.ContainerInspect(ctx, id)
			assert.NilError(t, err)
			assert.Equal(t, inspect.State.ExitCode, d.expectedExitCode)
		})
	}
}

// TestStopContainerWithTimeoutCancel checks that ContainerStop is not cancelled
// if the request is cancelled.
// See issue https://github.com/moby/moby/issues/45731
func TestStopContainerWithTimeoutCancel(t *testing.T) {
	t.Parallel()
	defer setupTest(t)()
	apiClient := testEnv.APIClient()
	t.Cleanup(func() { _ = apiClient.Close() })

	ctx := context.Background()
	id := container.Run(ctx, t, apiClient,
		container.WithCmd("sh", "-c", "trap 'echo received TERM' TERM; while true; do usleep 10; done"),
	)
	poll.WaitOn(t, container.IsInState(ctx, apiClient, id, "running"))

	ctxCancel, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	const stopTimeout = 3

	stoppedCh := make(chan error)
	go func() {
		sto := stopTimeout
		stoppedCh <- apiClient.ContainerStop(ctxCancel, id, containertypes.StopOptions{Timeout: &sto})
	}()

	poll.WaitOn(t, logsContains(ctx, apiClient, id, "received TERM"))

	// Cancel the context once we verified the container was signaled, and check
	// that the container is not killed immediately
	cancel()

	select {
	case stoppedErr := <-stoppedCh:
		assert.Check(t, is.ErrorType(stoppedErr, errdefs.IsCancelled))
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for stop request to be cancelled")
	}
	inspect, err := apiClient.ContainerInspect(ctx, id)
	assert.Check(t, err)
	assert.Check(t, inspect.State.Running)

	// container should be stopped after stopTimeout is reached. The daemon.containerStop
	// code is rather convoluted, and waits another 2 seconds for the container to
	// terminate after signaling it;
	// https://github.com/moby/moby/blob/97455cc31ffa08078db6591f018256ed59c35bbc/daemon/stop.go#L101-L112
	//
	// Adding 3 seconds to the specified stopTimeout to take this into account,
	// and add another second margin to try to avoid flakiness.
	poll.WaitOn(t, container.IsStopped(ctx, apiClient, id), poll.WithTimeout((3+stopTimeout)*time.Second))
}

// logsContains verifies the container contains the given text in the log's stdout.
func logsContains(ctx context.Context, client client.APIClient, containerID string, logString string) func(log poll.LogT) poll.Result {
	return func(log poll.LogT) poll.Result {
		logs, err := client.ContainerLogs(ctx, containerID, types.ContainerLogsOptions{
			ShowStdout: true,
		})
		if err != nil {
			return poll.Error(err)
		}
		defer logs.Close()

		var stdout bytes.Buffer
		_, err = stdcopy.StdCopy(&stdout, io.Discard, logs)
		if err != nil {
			return poll.Error(err)
		}
		if strings.Contains(stdout.String(), logString) {
			return poll.Success()
		}
		return poll.Continue("waiting for logstring '%s' in container", logString)
	}
}

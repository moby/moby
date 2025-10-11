package container

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/client"
	"github.com/moby/moby/client/pkg/stdcopy"
	"github.com/moby/moby/v2/integration/internal/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
)

// TestStopContainerWithTimeoutCancel checks that ContainerStop is not cancelled
// if the request is cancelled.
// See issue https://github.com/moby/moby/issues/45731
func TestStopContainerWithTimeoutCancel(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()
	t.Cleanup(func() { _ = apiClient.Close() })

	t.Parallel()

	id := container.Run(ctx, t, apiClient,
		container.WithCmd("sh", "-c", "trap 'echo received TERM' TERM; while true; do usleep 10; done"),
	)

	ctxCancel, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	const stopTimeout = 3

	stoppedCh := make(chan error)
	go func() {
		sto := stopTimeout
		stoppedCh <- apiClient.ContainerStop(ctxCancel, id, client.ContainerStopOptions{Timeout: &sto})
	}()

	poll.WaitOn(t, logsContains(ctx, apiClient, id, "received TERM"))

	// Cancel the context once we verified the container was signaled, and check
	// that the container is not killed immediately
	cancel()

	select {
	case stoppedErr := <-stoppedCh:
		assert.Check(t, is.ErrorType(stoppedErr, cerrdefs.IsCanceled))
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
func logsContains(ctx context.Context, apiClient client.APIClient, containerID string, logString string) func(log poll.LogT) poll.Result {
	return func(log poll.LogT) poll.Result {
		logs, err := apiClient.ContainerLogs(ctx, containerID, client.ContainerLogsOptions{
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

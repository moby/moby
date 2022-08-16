package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestWaitNonBlocked(t *testing.T) {
	defer setupTest(t)()
	cli := request.NewAPIClient(t)

	testCases := []struct {
		doc          string
		cmd          string
		expectedCode int64
	}{
		{
			doc:          "wait-nonblocking-exit-0",
			cmd:          "exit 0",
			expectedCode: 0,
		},
		{
			doc:          "wait-nonblocking-exit-random",
			cmd:          "exit 99",
			expectedCode: 99,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.doc, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			containerID := container.Run(ctx, t, cli, container.WithCmd("sh", "-c", tc.cmd))
			poll.WaitOn(t, container.IsInState(ctx, cli, containerID, "exited"), poll.WithTimeout(30*time.Second), poll.WithDelay(100*time.Millisecond))

			waitResC, errC := cli.ContainerWait(ctx, containerID, "")
			select {
			case err := <-errC:
				assert.NilError(t, err)
			case waitRes := <-waitResC:
				assert.Check(t, is.Equal(tc.expectedCode, waitRes.StatusCode))
			}
		})
	}
}

func TestWaitBlocked(t *testing.T) {
	// Windows busybox does not support trap in this way, not sleep with sub-second
	// granularity. It will always exit 0x40010004.
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	defer setupTest(t)()
	cli := request.NewAPIClient(t)

	testCases := []struct {
		doc          string
		cmd          string
		expectedCode int64
	}{
		{
			doc:          "test-wait-blocked-exit-zero",
			cmd:          "trap 'exit 0' TERM; while true; do usleep 10; done",
			expectedCode: 0,
		},
		{
			doc:          "test-wait-blocked-exit-random",
			cmd:          "trap 'exit 99' TERM; while true; do usleep 10; done",
			expectedCode: 99,
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.doc, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			containerID := container.Run(ctx, t, cli, container.WithCmd("sh", "-c", tc.cmd))
			poll.WaitOn(t, container.IsInState(ctx, cli, containerID, "running"), poll.WithTimeout(30*time.Second), poll.WithDelay(100*time.Millisecond))

			waitResC, errC := cli.ContainerWait(ctx, containerID, "")

			err := cli.ContainerStop(ctx, containerID, containertypes.StopOptions{})
			assert.NilError(t, err)

			select {
			case err := <-errC:
				assert.NilError(t, err)
			case waitRes := <-waitResC:
				assert.Check(t, is.Equal(tc.expectedCode, waitRes.StatusCode))
			case <-time.After(2 * time.Second):
				t.Fatal("timeout waiting for `docker wait`")
			}
		})
	}
}

func TestWaitConditions(t *testing.T) {
	defer setupTest(t)()
	cli := request.NewAPIClient(t)

	testCases := []struct {
		doc          string
		waitCond     containertypes.WaitCondition
		expectedCode int64
	}{
		{
			doc:          "default",
			expectedCode: 99,
		},
		{
			doc:          "not-running",
			expectedCode: 99,
			waitCond:     containertypes.WaitConditionNotRunning,
		},
		{
			doc:          "next-exit",
			expectedCode: 99,
			waitCond:     containertypes.WaitConditionNextExit,
		},
		{
			doc:          "removed",
			expectedCode: 99,
			waitCond:     containertypes.WaitConditionRemoved,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.doc, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			opts := []func(*container.TestContainerConfig){
				container.WithCmd("sh", "-c", "sleep 1; exit 99"),
			}
			if tc.waitCond == containertypes.WaitConditionRemoved {
				opts = append(opts, container.WithAutoRemove)
			}
			containerID := container.Run(ctx, t, cli, opts...)
			poll.WaitOn(t, container.IsInState(ctx, cli, containerID, "running"), poll.WithTimeout(30*time.Second), poll.WithDelay(100*time.Millisecond))

			waitResC, errC := cli.ContainerWait(ctx, containerID, tc.waitCond)
			select {
			case err := <-errC:
				assert.NilError(t, err)
			case waitRes := <-waitResC:
				assert.Check(t, is.Equal(tc.expectedCode, waitRes.StatusCode))
			}
		})
	}
}

func TestWaitRestartedContainer(t *testing.T) {
	defer setupTest(t)()
	cli := request.NewAPIClient(t)

	testCases := []struct {
		doc      string
		waitCond containertypes.WaitCondition
	}{
		{
			doc: "default",
		},
		{
			doc:      "not-running",
			waitCond: containertypes.WaitConditionNotRunning,
		},
		{
			doc:      "next-exit",
			waitCond: containertypes.WaitConditionNextExit,
		},
	}

	// We can't catch the SIGTERM in the Windows based busybox image
	isWindowDaemon := testEnv.DaemonInfo.OSType == "windows"

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.doc, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			containerID := container.Run(ctx, t, cli,
				container.WithCmd("sh", "-c", "trap 'exit 5' SIGTERM; while true; do sleep 0.1; done"),
			)
			defer cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{Force: true})

			poll.WaitOn(t, container.IsInState(ctx, cli, containerID, "running"), poll.WithTimeout(30*time.Second), poll.WithDelay(100*time.Millisecond))

			// Container is running now, wait for exit
			waitResC, errC := cli.ContainerWait(ctx, containerID, tc.waitCond)

			timeout := 5
			// On Windows it will always timeout, because our process won't receive SIGTERM
			// Skip to force killing immediately
			if isWindowDaemon {
				timeout = 0
			}

			err := cli.ContainerRestart(ctx, containerID, containertypes.StopOptions{Timeout: &timeout, Signal: "SIGTERM"})
			assert.NilError(t, err)

			select {
			case err := <-errC:
				t.Fatalf("Unexpected error: %v", err)
			case <-time.After(time.Second * 3):
				t.Fatalf("Wait should end after restart")
			case waitRes := <-waitResC:
				expectedCode := int64(5)

				if !isWindowDaemon {
					assert.Check(t, is.Equal(expectedCode, waitRes.StatusCode))
				}
			}
		})
	}

}

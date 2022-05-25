package container // import "github.com/docker/docker/integration/container"

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/pkg/stdcopy"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

// Regression test for #35370
// Makes sure that when following we don't get an EOF error when there are no logs
func TestLogsFollowTailEmpty(t *testing.T) {
	// FIXME(vdemeester) fails on a e2e run on linux...
	skip.If(t, testEnv.IsRemoteDaemon)
	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	id := container.Run(ctx, t, client, container.WithCmd("sleep", "100000"))

	logs, err := client.ContainerLogs(ctx, id, types.ContainerLogsOptions{ShowStdout: true, Tail: "2"})
	if logs != nil {
		defer logs.Close()
	}
	assert.Check(t, err)

	_, err = stdcopy.StdCopy(io.Discard, io.Discard, logs)
	assert.Check(t, err)
}

// Containers without the Tty have their stdout and stderr muxed
// into one stream with stdcopy
func TestLogsMuxed(t *testing.T) {
	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	testCases := []struct {
		desc        string
		logOps      types.ContainerLogsOptions
		expectedOut string
		expectedErr string
	}{
		{
			desc: "stdout and stderr",
			logOps: types.ContainerLogsOptions{
				ShowStdout: true,
				ShowStderr: true,
			},
			expectedOut: "this is fine",
			expectedErr: "accidents happen",
		},
		{
			desc: "only stdout",
			logOps: types.ContainerLogsOptions{
				ShowStdout: true,
				ShowStderr: false,
			},
			expectedOut: "this is fine",
			expectedErr: "",
		},
		{
			desc: "only stderr",
			logOps: types.ContainerLogsOptions{
				ShowStdout: false,
				ShowStderr: true,
			},
			expectedOut: "",
			expectedErr: "accidents happen",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			id := container.Run(ctx, t, client,
				container.WithTty(false), // Stream is muxed when not TTY
				container.WithCmd("sh", "-c", "echo -n this is fine; echo -n accidents happen >&2"),
			)
			defer client.ContainerRemove(ctx, id, types.ContainerRemoveOptions{Force: true})

			poll.WaitOn(t, container.IsStopped(ctx, client, id), poll.WithDelay(time.Millisecond*100))

			logs, err := client.ContainerLogs(ctx, id, tC.logOps)
			if logs != nil {
				defer logs.Close()
			}
			assert.NilError(t, err)

			stderr := new(bytes.Buffer)
			stdout := new(bytes.Buffer)

			_, err = stdcopy.StdCopy(stdout, stderr, logs)
			assert.NilError(t, err)
			assert.Equal(t, stdout.String(), tC.expectedOut)
			assert.Equal(t, stderr.String(), tC.expectedErr)
		})
	}

}

// Containers with tty have their stdout and stderr both redirected to one output stream
// This makes all the logs saved as they were printed to stdout
func TestLogsNotMuxed(t *testing.T) {
	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	testCases := []struct {
		desc           string
		logOps         types.ContainerLogsOptions
		expectedOutput string
	}{
		{
			desc: "stdout and stderr",
			logOps: types.ContainerLogsOptions{
				ShowStdout: true,
				ShowStderr: true,
			},
			expectedOutput: "this is fineaccidents happen",
		},
		{
			desc: "only stdout",
			logOps: types.ContainerLogsOptions{
				ShowStdout: true,
				ShowStderr: false,
			},
			expectedOutput: "this is fineaccidents happen",
		},
		{
			desc: "only stderr",
			logOps: types.ContainerLogsOptions{
				ShowStdout: false,
				ShowStderr: true,
			},
			expectedOutput: "",
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			id := container.Run(ctx, t, client,
				container.WithTty(true), // Stream is only stdout when TTY
				container.WithCmd("sh", "-c", "echo -n this is fine; echo -n accidents happen >&2"),
			)
			defer client.ContainerRemove(ctx, id, types.ContainerRemoveOptions{Force: true})

			poll.WaitOn(t, container.IsStopped(ctx, client, id), poll.WithDelay(time.Millisecond*100))

			logs, err := client.ContainerLogs(ctx, id, tC.logOps)
			if logs != nil {
				defer logs.Close()
			}
			assert.Check(t, err)

			out := new(bytes.Buffer)

			_, err = io.Copy(out, logs)
			assert.NilError(t, err)
			assert.Equal(t, out.String(), tC.expectedOutput)
		})
	}
}

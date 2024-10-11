package container // import "github.com/docker/docker/integration/container"

import (
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	systemutil "github.com/docker/docker/integration/internal/system"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestAttach(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	tests := []struct {
		doc               string
		tty               bool
		expectedMediaType string
	}{
		{
			doc:               "without TTY",
			expectedMediaType: types.MediaTypeMultiplexedStream,
		},
		{
			doc:               "with TTY",
			tty:               true,
			expectedMediaType: types.MediaTypeRawStream,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.doc, func(t *testing.T) {
			t.Parallel()

			ctx := testutil.StartSpan(ctx, t)
			resp, err := apiClient.ContainerCreate(ctx,
				&container.Config{
					Image: "busybox",
					Cmd:   []string{"echo", "hello"},
					Tty:   tc.tty,
				},
				&container.HostConfig{},
				&network.NetworkingConfig{},
				nil,
				"",
			)
			assert.NilError(t, err)
			attach, err := apiClient.ContainerAttach(ctx, resp.ID, container.AttachOptions{
				Stdout: true,
				Stderr: true,
			})
			assert.NilError(t, err)
			mediaType, ok := attach.MediaType()
			assert.Check(t, ok)
			assert.Check(t, is.Equal(mediaType, tc.expectedMediaType))
		})
	}
}

// Regression test for #37182
func TestAttachDisconnectLeak(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux", "Bug still exists on Windows")
	t.Parallel()

	ctx := testutil.StartSpan(baseContext, t)

	// Use a new daemon to make sure stuff from other tests isn't affecting the
	// goroutine count.
	d := daemon.New(t)
	defer d.Cleanup(t)

	d.StartWithBusybox(ctx, t, "--iptables=false", "--ip6tables=false")

	client := d.NewClientT(t)

	resp, err := client.ContainerCreate(ctx,
		&container.Config{
			Image: "busybox",
			Cmd:   []string{"/bin/sh", "-c", "while true; usleep 100000; done"},
		},
		&container.HostConfig{},
		&network.NetworkingConfig{},
		nil,
		"",
	)
	assert.NilError(t, err)
	cID := resp.ID
	defer client.ContainerRemove(ctx, cID, container.RemoveOptions{
		Force: true,
	})

	nGoroutines := systemutil.WaitForStableGoroutineCount(ctx, t, client)

	attach, err := client.ContainerAttach(ctx, cID, container.AttachOptions{
		Stdout: true,
	})
	assert.NilError(t, err)
	defer attach.Close()

	poll.WaitOn(t, func(_ poll.LogT) poll.Result {
		count := systemutil.WaitForStableGoroutineCount(ctx, t, client)
		if count > nGoroutines {
			return poll.Success()
		}
		return poll.Continue("waiting for goroutines to increase from %d, current: %d", nGoroutines, count)
	},
		poll.WithTimeout(time.Minute),
	)

	attach.Close()

	poll.WaitOn(t, systemutil.CheckGoroutineCount(ctx, client, nGoroutines), poll.WithTimeout(time.Minute))
}

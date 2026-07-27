package container

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/moby/moby/api/types"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	systemutil "github.com/moby/moby/v2/integration/internal/system"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/daemon"
	"github.com/moby/moby/v2/internal/testutil/request"
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
		t.Run(tc.doc, func(t *testing.T) {
			t.Parallel()

			ctx := testutil.StartSpan(ctx, t)
			resp, err := apiClient.ContainerCreate(ctx, client.ContainerCreateOptions{
				Config: &container.Config{
					Image: "busybox",
					Cmd:   []string{"echo", "hello"},
					Tty:   tc.tty,
				},
				HostConfig:       &container.HostConfig{},
				NetworkingConfig: &network.NetworkingConfig{},
			})
			assert.NilError(t, err)
			attach, err := apiClient.ContainerAttach(ctx, resp.ID, client.ContainerAttachOptions{
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

func TestAttachRejectsInvalidDetachKeys(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	resp, err := apiClient.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config: &container.Config{
			Image: "busybox",
			Cmd:   []string{"top"},
			Tty:   true,
		},
		HostConfig:       &container.HostConfig{},
		NetworkingConfig: &network.NetworkingConfig{},
	})
	assert.NilError(t, err)

	query := url.Values{"detachKeys": {"ctrl-A,a"}}
	res, body, err := request.Post(ctx, "/containers/"+resp.ID+"/attach?"+query.Encode(),
		request.With(func(req *http.Request) error {
			req.Header.Set("Connection", "Upgrade")
			req.Header.Set("Upgrade", "tcp")
			return nil
		}),
	)
	if body != nil {
		defer func() { _ = body.Close() }()
	}
	assert.NilError(t, err)
	assert.Equal(t, res.StatusCode, http.StatusBadRequest)

	out, err := request.ReadBody(body)
	assert.NilError(t, err)
	assert.Check(t, is.Contains(string(out), "Invalid detach keys"))
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

	apiClient := d.NewClientT(t)

	resp, err := apiClient.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config: &container.Config{
			Image: "busybox",
			Cmd:   []string{"/bin/sh", "-c", "while true; usleep 100000; done"},
		},
		HostConfig:       &container.HostConfig{},
		NetworkingConfig: &network.NetworkingConfig{},
	})
	assert.NilError(t, err)
	cID := resp.ID
	defer apiClient.ContainerRemove(ctx, cID, client.ContainerRemoveOptions{
		Force: true,
	})

	nGoroutines := systemutil.WaitForStableGoroutineCount(ctx, t, apiClient)

	attach, err := apiClient.ContainerAttach(ctx, cID, client.ContainerAttachOptions{
		Stdout: true,
	})
	assert.NilError(t, err)
	defer attach.Close()

	poll.WaitOn(t, func(_ poll.LogT) poll.Result {
		count := systemutil.WaitForStableGoroutineCount(ctx, t, apiClient)
		if count > nGoroutines {
			return poll.Success()
		}
		return poll.Continue("waiting for goroutines to increase from %d, current: %d", nGoroutines, count)
	},
		poll.WithTimeout(time.Minute),
	)

	attach.Close()

	poll.WaitOn(t, systemutil.CheckGoroutineCount(ctx, apiClient, nGoroutines), poll.WithTimeout(time.Minute))
}

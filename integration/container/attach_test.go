package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/testutil"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
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

	setupTest(t)
	client := testEnv.APIClient()

	resp, err := client.ContainerCreate(context.Background(),
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
	defer client.ContainerRemove(context.Background(), cID, container.RemoveOptions{
		Force: true,
	})

	info, err := client.Info(context.Background())
	assert.NilError(t, err)
	assert.Assert(t, info.NGoroutines > 1)

	attach, err := client.ContainerAttach(context.Background(), cID, container.AttachOptions{
		Stdout: true,
	})
	assert.NilError(t, err)
	defer attach.Close()

	infoAttach, err := client.Info(context.Background())
	assert.NilError(t, err)
	assert.Assert(t, infoAttach.NGoroutines > info.NGoroutines)

	attach.Close()

	var info2 system.Info
	for i := 0; i < 10; i++ {
		info2, err = client.Info(context.Background())
		assert.NilError(t, err)
		if info2.NGoroutines > info.NGoroutines {
			time.Sleep(time.Second)
			continue
		}
		return
	}

	t.Fatalf("goroutine leak: %d -> %d", info.NGoroutines, info2.NGoroutines)
}

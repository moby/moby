package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestAttach(t *testing.T) {
	t.Cleanup(setupTest(t))
	client := testEnv.APIClient()

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
			resp, err := client.ContainerCreate(context.Background(),
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
			attach, err := client.ContainerAttach(context.Background(), resp.ID, types.ContainerAttachOptions{
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

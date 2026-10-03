package containerd

import (
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	v2 "github.com/moby/moby/v2/daemon/pkg/plugin/v2"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestCheckPullDescriptorMediaType(t *testing.T) {
	for _, tc := range []struct {
		name        string
		mediaType   string
		expectError string
	}{
		{
			name:        "plugin config",
			mediaType:   v2.MediaTypePluginConfig,
			expectError: `cannot pull image: remote descriptor is a Docker plugin config ("application/vnd.docker.plugin.v1+json")`,
		},
		{
			name:        "unknown plugin config version",
			mediaType:   "application/vnd.docker.plugin.v0+json",
			expectError: `cannot pull image: remote descriptor is a Docker plugin config ("application/vnd.docker.plugin.v0+json")`,
		},
		{
			name:      "image config",
			mediaType: ocispec.MediaTypeImageConfig,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := checkPullDescriptorMediaType(ocispec.Descriptor{
				MediaType: tc.mediaType,
			})

			if tc.expectError == "" {
				assert.NilError(t, err)
				return
			}

			assert.Check(t, is.Error(err, tc.expectError))
			assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
		})
	}
}

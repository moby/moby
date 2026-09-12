package containerd

import (
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/distribution/manifest/schema2"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

const testPluginConfigError = `Encountered remote "application/vnd.docker.plugin.v1+json"(plugin) when fetching`

const (
	testLegacyPluginConfigMediaType = "application/vnd.docker.plugin.v0+json"
	testLegacyPluginConfigError     = `Encountered remote "application/vnd.docker.plugin.v0+json"(unknown) when fetching`
)

func TestCheckPullDescriptorMediaType(t *testing.T) {
	for _, tc := range []struct {
		name        string
		mediaType   string
		expectError string
	}{
		{
			name:        "plugin config",
			mediaType:   schema2.MediaTypePluginConfig,
			expectError: testPluginConfigError,
		},
		{
			name:        "legacy plugin config",
			mediaType:   testLegacyPluginConfigMediaType,
			expectError: testLegacyPluginConfigError,
		},
		{
			name:      "image config",
			mediaType: schema2.MediaTypeImageConfig,
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
			assert.Check(t, cerrdefs.IsInvalidArgument(err))
		})
	}
}

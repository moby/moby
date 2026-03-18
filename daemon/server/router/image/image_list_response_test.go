package image

import (
	"encoding/json"
	"testing"

	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/v2/daemon/internal/compat"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// TestImageListVirtualSize verifies that wrapping image summaries with
// compat.Wrap injects a VirtualSize field equal to Size for older API
// versions (< 1.44), and that VirtualSize is absent when the wrapper
// is not used.
func TestImageListVirtualSize(t *testing.T) {
	images := []image.Summary{
		{
			ID:          "sha256:abc123",
			RepoTags:    []string{"myimage:latest"},
			RepoDigests: []string{},
			Size:        12345,
			SharedSize:  -1,
			Containers:  -1,
		},
		{
			ID:          "sha256:def456",
			RepoTags:    []string{},
			RepoDigests: []string{"myimage@sha256:def456"},
			Size:        67890,
			SharedSize:  -1,
			Containers:  -1,
		},
	}

	t.Run("with VirtualSize (API < 1.44)", func(t *testing.T) {
		wrapped := make([]*compat.Wrapper, len(images))
		for i := range images {
			wrapped[i] = compat.Wrap(&images[i], compat.WithExtraFields(map[string]any{
				"VirtualSize": images[i].Size,
			}))
		}

		out, err := json.Marshal(wrapped)
		assert.NilError(t, err)

		var result []map[string]any
		err = json.Unmarshal(out, &result)
		assert.NilError(t, err)
		assert.Assert(t, is.Len(result, 2))

		for i, img := range result {
			vs, ok := img["VirtualSize"]
			assert.Assert(t, ok, "VirtualSize should be present for image %d", i)
			// json.Unmarshal decodes numbers as float64
			assert.Check(t, is.Equal(vs, float64(images[i].Size)),
				"VirtualSize should equal Size for image %d", i)

			sz, ok := img["Size"]
			assert.Assert(t, ok, "Size should be present for image %d", i)
			assert.Check(t, is.Equal(sz, float64(images[i].Size)),
				"Size should be preserved for image %d", i)
		}
	})

	t.Run("without VirtualSize (API >= 1.44)", func(t *testing.T) {
		out, err := json.Marshal(images)
		assert.NilError(t, err)

		var result []map[string]any
		err = json.Unmarshal(out, &result)
		assert.NilError(t, err)
		assert.Assert(t, is.Len(result, 2))

		for i, img := range result {
			_, ok := img["VirtualSize"]
			assert.Assert(t, !ok, "VirtualSize should NOT be present for image %d", i)
		}
	})
}

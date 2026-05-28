package system

import (
	"encoding/json"
	"testing"

	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/v2/daemon/internal/compat"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// TestDiskUsageVirtualSize verifies that wrapping the legacyDiskUsage
// struct with compat.Wrap injects VirtualSize into each image summary
// for older API versions (< 1.44), and that VirtualSize is absent when
// the wrapper is not used.
func TestDiskUsageVirtualSize(t *testing.T) {
	legacy := legacyDiskUsage{
		LayersSize: 100000,
		Images: []image.Summary{
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
		},
	}

	t.Run("with VirtualSize (API < 1.44)", func(t *testing.T) {
		wrappedImages := make([]*compat.Wrapper, len(legacy.Images))
		for i := range legacy.Images {
			wrappedImages[i] = compat.Wrap(&legacy.Images[i], compat.WithExtraFields(map[string]any{
				"VirtualSize": legacy.Images[i].Size,
			}))
		}

		wrapped := compat.Wrap(&legacy,
			compat.WithOmittedFields("Images"),
			compat.WithExtraFields(map[string]any{
				"Images": wrappedImages,
			}),
		)

		out, err := json.Marshal(wrapped)
		assert.NilError(t, err)

		var result map[string]any
		err = json.Unmarshal(out, &result)
		assert.NilError(t, err)

		imagesRaw, ok := result["Images"]
		assert.Assert(t, ok, "Images field should be present")

		imagesList, ok := imagesRaw.([]any)
		assert.Assert(t, ok, "Images should be an array")
		assert.Assert(t, is.Len(imagesList, 2))

		for i, imgRaw := range imagesList {
			img, ok := imgRaw.(map[string]any)
			assert.Assert(t, ok, "each image should be an object")

			vs, ok := img["VirtualSize"]
			assert.Assert(t, ok, "VirtualSize should be present for image %d", i)
			assert.Check(t, is.Equal(vs, float64(legacy.Images[i].Size)),
				"VirtualSize should equal Size for image %d", i)

			sz, ok := img["Size"]
			assert.Assert(t, ok, "Size should be present for image %d", i)
			assert.Check(t, is.Equal(sz, float64(legacy.Images[i].Size)),
				"Size should be preserved for image %d", i)
		}

		// LayersSize should still be present
		ls, ok := result["LayersSize"]
		assert.Assert(t, ok, "LayersSize should be present")
		assert.Check(t, is.Equal(ls, float64(100000)))
	})

	t.Run("without VirtualSize (API >= 1.44)", func(t *testing.T) {
		out, err := json.Marshal(&legacy)
		assert.NilError(t, err)

		var result map[string]any
		err = json.Unmarshal(out, &result)
		assert.NilError(t, err)

		imagesRaw, ok := result["Images"]
		assert.Assert(t, ok, "Images field should be present")

		imagesList, ok := imagesRaw.([]any)
		assert.Assert(t, ok, "Images should be an array")
		assert.Assert(t, is.Len(imagesList, 2))

		for i, imgRaw := range imagesList {
			img, ok := imgRaw.(map[string]any)
			assert.Assert(t, ok, "each image should be an object")

			_, ok = img["VirtualSize"]
			assert.Assert(t, !ok, "VirtualSize should NOT be present for image %d", i)
		}
	})

	t.Run("empty images list", func(t *testing.T) {
		emptyLegacy := legacyDiskUsage{
			LayersSize: 0,
			Images:     []image.Summary{},
		}

		// Even with the wrapping logic, empty images should produce an empty array
		wrappedImages := make([]*compat.Wrapper, 0)
		wrapped := compat.Wrap(&emptyLegacy,
			compat.WithOmittedFields("Images"),
			compat.WithExtraFields(map[string]any{
				"Images": wrappedImages,
			}),
		)

		out, err := json.Marshal(wrapped)
		assert.NilError(t, err)

		var result map[string]any
		err = json.Unmarshal(out, &result)
		assert.NilError(t, err)

		imagesRaw, ok := result["Images"]
		assert.Assert(t, ok, "Images field should be present")
		imagesList, ok := imagesRaw.([]any)
		assert.Assert(t, ok, "Images should be an array")
		assert.Assert(t, is.Len(imagesList, 0))
	})
}

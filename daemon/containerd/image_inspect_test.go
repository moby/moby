package containerd

import (
	"fmt"
	"path/filepath"
	"testing"

	c8dimages "github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/log/logtest"
	"github.com/moby/moby/v2/daemon/server/imagebackend"
	"github.com/moby/moby/v2/internal/testutil/specialimage"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestImageInspect(t *testing.T) {
	ctx := namespaces.WithNamespace(t.Context(), "testing")

	blobsDir := t.TempDir()

	toContainerdImage := func(t *testing.T, imageFunc specialimage.SpecialImageFunc) c8dimages.Image {
		idx, err := imageFunc(blobsDir)
		assert.NilError(t, err)

		return imagesFromIndex(idx)[0]
	}

	missingMultiPlatform := toContainerdImage(t, func(dir string) (*ocispec.Index, error) {
		idx, _, err := specialimage.PartialMultiPlatform(dir, "missingmp:latest", specialimage.PartialOpts{
			Stored: nil,
			Missing: []ocispec.Platform{
				{OS: "linux", Architecture: "arm64"},
				{OS: "linux", Architecture: "amd64"},
			},
		})
		return idx, err
	})

	cs := &blobsDirContentStore{blobs: filepath.Join(blobsDir, "blobs/sha256")}

	t.Run("inspect image with manifests but missing platform blobs", func(t *testing.T) {
		ctx := logtest.WithT(ctx, t)
		service := fakeImageService(t, ctx, cs)

		_, err := service.images.Create(ctx, missingMultiPlatform)
		assert.NilError(t, err)

		for _, manifests := range []bool{true, false} {
			t.Run(fmt.Sprintf("manifests=%t", manifests), func(t *testing.T) {
				inspect, err := service.ImageInspect(ctx, missingMultiPlatform.Name, imagebackend.ImageInspectOpts{Manifests: manifests})
				assert.NilError(t, err)

				if manifests {
					assert.Check(t, is.Len(inspect.Manifests, 2))
				} else {
					assert.Check(t, is.Len(inspect.Manifests, 0))
				}
			})
		}
	})

	t.Run("inspect image with one layer missing", func(t *testing.T) {
		ctx := logtest.WithT(ctx, t)
		service := fakeImageService(t, ctx, cs)

		img := toContainerdImage(t, specialimage.MultiLayer)

		_, err := service.images.Create(ctx, img)
		assert.NilError(t, err)

		// Get the manifest to access the layers
		mfst, err := c8dimages.Manifest(ctx, cs, img.Target, nil)
		assert.NilError(t, err)
		assert.Check(t, len(mfst.Layers) > 0, "image should have at least one layer")

		// Delete the last layer from the content store
		lastLayer := mfst.Layers[len(mfst.Layers)-1]
		err = cs.Delete(ctx, lastLayer.Digest)
		assert.NilError(t, err)

		inspect, err := service.ImageInspect(ctx, img.Name, imagebackend.ImageInspectOpts{})
		assert.NilError(t, err)

		assert.Check(t, inspect.Config != nil)
		assert.Check(t, is.Len(inspect.RootFS.Layers, len(mfst.Layers)))
	})

	t.Run("inspect image with platform parameter", func(t *testing.T) {
		ctx := logtest.WithT(ctx, t)
		service := fakeImageService(t, ctx, cs)

		multiPlatformImage := toContainerdImage(t, func(dir string) (*ocispec.Index, error) {
			idx, _, err := specialimage.MultiPlatform(dir, "multiplatform:latest", []ocispec.Platform{
				{OS: "linux", Architecture: "amd64"},
				{OS: "linux", Architecture: "arm64"},
			})
			return idx, err
		})

		_, err := service.images.Create(ctx, multiPlatformImage)
		assert.NilError(t, err)

		// Test with amd64 platform
		amd64Platform := &ocispec.Platform{OS: "linux", Architecture: "amd64"}
		inspectAmd64, err := service.ImageInspect(ctx, multiPlatformImage.Name, imagebackend.ImageInspectOpts{
			Platform: amd64Platform,
		})
		assert.NilError(t, err)
		assert.Equal(t, inspectAmd64.Architecture, "amd64")
		assert.Equal(t, inspectAmd64.Os, "linux")

		// Test with arm64 platform
		arm64Platform := &ocispec.Platform{OS: "linux", Architecture: "arm64"}
		inspectArm64, err := service.ImageInspect(ctx, multiPlatformImage.Name, imagebackend.ImageInspectOpts{
			Platform: arm64Platform,
		})
		assert.NilError(t, err)
		assert.Equal(t, inspectArm64.Architecture, "arm64")
		assert.Equal(t, inspectArm64.Os, "linux")
	})
}

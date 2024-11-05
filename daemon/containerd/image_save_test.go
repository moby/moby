package containerd

import (
	"context"
	"io"
	"path/filepath"
	"testing"

	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/platforms"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/internal/testutils/specialimage"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestImageMultiplatformSaveShallowWithNative(t *testing.T) {
	ctx := namespaces.WithNamespace(context.TODO(), "testing-"+t.Name())

	contentDir := t.TempDir()
	store := &blobsDirContentStore{blobs: filepath.Join(contentDir, "blobs/sha256")}

	native := platforms.Platform{
		OS:           "linux",
		Architecture: "amd64",
	}

	arm64 := platforms.Platform{
		OS:           "linux",
		Architecture: "arm64",
	}

	imgSvc := fakeImageService(t, ctx, store)
	// Mock the native platform.
	imgSvc.defaultPlatformOverride = platforms.Only(native)

	idx, _, err := specialimage.PartialMultiPlatform(contentDir, "partial-with-native:latest", specialimage.PartialOpts{
		Stored:  []ocispec.Platform{native},
		Missing: []ocispec.Platform{arm64},
	})
	assert.NilError(t, err)

	img, err := imgSvc.images.Create(ctx, imagesFromIndex(idx)[0])
	assert.NilError(t, err)

	t.Run("export without specific platform", func(t *testing.T) {
		err = imgSvc.ExportImage(ctx, []string{img.Name}, nil, io.Discard)
		assert.NilError(t, err)
	})
	t.Run("export native", func(t *testing.T) {
		err = imgSvc.ExportImage(ctx, []string{img.Name}, &native, io.Discard)
		assert.NilError(t, err)
	})
	t.Run("export missing", func(t *testing.T) {
		err = imgSvc.ExportImage(ctx, []string{img.Name}, &arm64, io.Discard)
		assert.Check(t, is.ErrorType(err, errdefs.IsNotFound))
	})
}

func TestImageMultiplatformSaveShallowWithoutNative(t *testing.T) {
	ctx := namespaces.WithNamespace(context.TODO(), "testing-"+t.Name())

	contentDir := t.TempDir()
	store := &blobsDirContentStore{blobs: filepath.Join(contentDir, "blobs/sha256")}

	native := platforms.Platform{
		OS:           "linux",
		Architecture: "amd64",
	}

	arm64 := platforms.Platform{
		OS:           "linux",
		Architecture: "arm64",
	}

	imgSvc := fakeImageService(t, ctx, store)
	// Mock the native platform.
	imgSvc.defaultPlatformOverride = platforms.Only(native)

	idx, _, err := specialimage.PartialMultiPlatform(contentDir, "partial-without-native:latest", specialimage.PartialOpts{
		Stored:  []ocispec.Platform{arm64},
		Missing: []ocispec.Platform{native},
	})
	assert.NilError(t, err)

	img, err := imgSvc.images.Create(ctx, imagesFromIndex(idx)[0])
	assert.NilError(t, err)

	t.Run("export without specific platform", func(t *testing.T) {
		t.Skip("TODO(vvoland): https://github.com/docker/cli/issues/5476")
		err = imgSvc.ExportImage(ctx, []string{img.Name}, nil, io.Discard)
		assert.NilError(t, err)
	})
	t.Run("export native", func(t *testing.T) {
		err = imgSvc.ExportImage(ctx, []string{img.Name}, &native, io.Discard)
		assert.Check(t, is.ErrorType(err, errdefs.IsNotFound))
	})
	t.Run("export arm64", func(t *testing.T) {
		err = imgSvc.ExportImage(ctx, []string{img.Name}, &arm64, io.Discard)
		assert.NilError(t, err)
	})
}

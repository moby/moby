package containerd

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/plugins/content/local"
	"github.com/containerd/platforms"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/internal/testutils/specialimage"
	"github.com/docker/docker/pkg/archive"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestImageLoadMissing(t *testing.T) {
	linuxAmd64 := ocispec.Platform{OS: "linux", Architecture: "amd64"}
	linuxArm64 := ocispec.Platform{OS: "linux", Architecture: "arm64"}
	linuxArmv5 := ocispec.Platform{OS: "linux", Architecture: "arm", Variant: "v5"}

	ctx := namespaces.WithNamespace(context.TODO(), "testing-"+t.Name())

	store, err := local.NewLabeledStore(t.TempDir(), &memoryLabelStore{})
	assert.NilError(t, err)

	imgSvc := fakeImageService(t, ctx, store)
	// Mock the daemon platform.
	imgSvc.defaultPlatformOverride = platforms.Only(linuxAmd64)

	tryLoad := func(ctx context.Context, t *testing.T, dir string, platform ocispec.Platform) error {
		tarRc, err := archive.Tar(dir, archive.Uncompressed)
		assert.NilError(t, err)
		defer tarRc.Close()

		buf := bytes.Buffer{}

		defer func() {
			t.Log(buf.String())
		}()

		return imgSvc.LoadImage(ctx, tarRc, &platform, &buf, true)
	}

	clearStore := func(ctx context.Context, t *testing.T) {
		assert.NilError(t, store.Walk(ctx, func(info content.Info) error {
			return store.Delete(ctx, info.Digest)
		}), "failed to delete all content")
	}

	t.Run("empty index", func(t *testing.T) {
		imgDataDir := t.TempDir()
		_, err := specialimage.EmptyIndex(imgDataDir)
		assert.NilError(t, err)

		err = tryLoad(ctx, t, imgDataDir, linuxAmd64)
		assert.Check(t, is.Error(err, "image emptyindex:latest was loaded, but doesn't provide the requested platform (linux/amd64)"))
		assert.Check(t, is.ErrorType(err, errdefs.IsNotFound))
	})
	clearStore(ctx, t)

	t.Run("single platform", func(t *testing.T) {
		imgDataDir := t.TempDir()
		r := rand.NewSource(0x9127371238)
		_, err := specialimage.RandomSinglePlatform(imgDataDir, linuxAmd64, r)
		assert.NilError(t, err)

		err = tryLoad(ctx, t, imgDataDir, linuxArm64)
		assert.Check(t, is.ErrorContains(err, "doesn't provide the requested platform (linux/arm64)"))
		assert.Check(t, is.ErrorType(err, errdefs.IsNotFound))
	})

	clearStore(ctx, t)

	t.Run("2 platform image", func(t *testing.T) {
		imgDataDir := t.TempDir()
		_, mfstDescs, err := specialimage.MultiPlatform(imgDataDir, "multiplatform:latest", []ocispec.Platform{linuxAmd64, linuxArm64})
		assert.NilError(t, err)

		t.Run("platform not included in index", func(t *testing.T) {
			err = tryLoad(ctx, t, imgDataDir, linuxArmv5)
			assert.Check(t, is.Error(err, "image multiplatform:latest was loaded, but doesn't provide the requested platform (linux/arm/v5)"))
			assert.Check(t, is.ErrorType(err, errdefs.IsNotFound))
		})

		clearStore(ctx, t)

		t.Run("platform blobs missing", func(t *testing.T) {
			// Assumption: arm64 image is second in the index (implementation detail of specialimage.MultiPlatform)
			mfstDesc := mfstDescs[1]
			assert.Assert(t, mfstDesc.Platform.Architecture == linuxArm64.Architecture)
			assert.Assert(t, mfstDesc.Platform.Variant == linuxArm64.Variant)

			t.Log(mfstDesc.Digest)

			// Delete arm64 manifest
			mfstPath := filepath.Join(imgDataDir, "blobs/sha256", mfstDesc.Digest.Encoded())
			assert.NilError(t, os.Remove(mfstPath))

			err = tryLoad(ctx, t, imgDataDir, linuxArm64)
			assert.Check(t, is.ErrorContains(err, "requested platform (linux/arm64) found, but some content is missing"))
			assert.Check(t, is.ErrorType(err, errdefs.IsNotFound))
		})
	})
}

func TestImageLoadDangling(t *testing.T) {
	linuxAmd64 := ocispec.Platform{OS: "linux", Architecture: "amd64"}

	ctx := namespaces.WithNamespace(context.TODO(), "testing-"+t.Name())

	store, err := local.NewLabeledStore(t.TempDir(), &memoryLabelStore{})
	assert.NilError(t, err)

	imgSvc := fakeImageService(t, ctx, store)
	// Mock the daemon platform.
	imgSvc.defaultPlatformOverride = platforms.Only(linuxAmd64)

	tryLoad := func(ctx context.Context, t *testing.T, dir string, platform ocispec.Platform) error {
		tarRc, err := archive.Tar(dir, archive.Uncompressed)
		assert.NilError(t, err)
		defer tarRc.Close()

		buf := bytes.Buffer{}

		defer func() {
			t.Log(buf.String())
		}()

		return imgSvc.LoadImage(ctx, tarRc, &platform, &buf, true)
	}

	imgDataDir := t.TempDir()
	r := rand.NewSource(0x9127371238)
	_, err = specialimage.RandomSinglePlatform(imgDataDir, linuxAmd64, r)
	assert.NilError(t, err)

	err = tryLoad(ctx, t, imgDataDir, linuxAmd64)
	assert.NilError(t, err)

	// Use containerd client to verify the dangling image exists.
	c8dimgs, err := imgSvc.client.ListImages(ctx)
	assert.NilError(t, err)
	assert.Check(t, is.Len(c8dimgs, 2))

	// Both images should refer to the same digest.
	dgsts := make(map[string]struct{})
	for _, img := range c8dimgs {
		dgsts[img.Target().Digest.String()] = struct{}{}
	}
	assert.Check(t, is.Len(dgsts, 1))

	// One of these should be tagged as moby-dangling.
	want := fmt.Sprintf("moby-dangling@%s", c8dimgs[0].Target().Digest)
	index := slices.IndexFunc(c8dimgs, func(img client.Image) bool {
		return img.Name() == want
	})
	assert.Check(t, index >= 0, "cannot find dangling image: %s", want)

	// Ensure image service still only returns one image.
	imgs, err := imgSvc.Images(ctx, image.ListOptions{All: true})
	assert.NilError(t, err)
	assert.Check(t, is.Len(imgs, 1))
}

package containerd

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/containerd/containerd/v2/core/content"
	c8dimages "github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/plugins/content/local"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/platforms"
	"github.com/moby/go-archive"
	"github.com/moby/go-archive/compression"
	"github.com/moby/moby/v2/daemon/server/imagebackend"
	"github.com/moby/moby/v2/internal/testutil/labelstore"
	"github.com/moby/moby/v2/internal/testutil/specialimage"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestImageLoad(t *testing.T) {
	linuxAmd64 := ocispec.Platform{OS: "linux", Architecture: "amd64"}
	linuxArm64 := ocispec.Platform{OS: "linux", Architecture: "arm64"}
	linuxArmv5 := ocispec.Platform{OS: "linux", Architecture: "arm", Variant: "v5"}
	linuxRiscv64 := ocispec.Platform{OS: "linux", Architecture: "riskv64"}

	ctx := namespaces.WithNamespace(t.Context(), "testing-"+t.Name())

	store, err := local.NewLabeledStore(t.TempDir(), &labelstore.InMemory{})
	assert.NilError(t, err)

	imgSvc := fakeImageService(t, ctx, store)
	// Mock the daemon platform.
	imgSvc.defaultPlatformOverride = &linuxAmd64

	tryLoad := func(ctx context.Context, t *testing.T, dir string, platformList []ocispec.Platform) error {
		tarRc, err := archive.Tar(dir, compression.None)
		assert.NilError(t, err)
		defer tarRc.Close()

		buf := bytes.Buffer{}

		defer func() {
			t.Log(buf.String())
		}()

		return imgSvc.LoadImage(ctx, tarRc, platformList, &buf, true)
	}

	cleanup := func(ctx context.Context, t *testing.T) {
		// Remove all existing images to start fresh
		images, err := imgSvc.Images(ctx, imagebackend.ListOptions{})
		assert.NilError(t, err)
		for _, img := range images {
			_, err := imgSvc.ImageDelete(ctx, img.ID, imagebackend.RemoveOptions{PruneChildren: true})
			assert.NilError(t, err)
		}

		// Remove all content from the store
		assert.NilError(t, store.Walk(ctx, func(info content.Info) error {
			return store.Delete(ctx, info.Digest)
		}), "failed to delete all content")
	}

	t.Run("empty index", func(t *testing.T) {
		imgDataDir := t.TempDir()
		_, err := specialimage.EmptyIndex(imgDataDir)
		assert.NilError(t, err)

		err = tryLoad(ctx, t, imgDataDir, []ocispec.Platform{linuxAmd64})
		assert.Check(t, is.Error(err, "image emptyindex:latest was loaded, but doesn't provide the requested platform ([linux/amd64])"))
		assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
	})
	cleanup(ctx, t)

	t.Run("single platform", func(t *testing.T) {
		imgDataDir := t.TempDir()
		r := rand.NewSource(0x9127371238)
		_, err = specialimage.RandomSinglePlatform(imgDataDir, linuxAmd64, r)
		assert.NilError(t, err)

		platforms := []ocispec.Platform{linuxAmd64}
		err = tryLoad(ctx, t, imgDataDir, platforms)
		assert.NilError(t, err)

		err = tryLoad(ctx, t, imgDataDir, []ocispec.Platform{linuxArm64})
		assert.Check(t, is.ErrorContains(err, "doesn't provide the requested platform ([linux/arm64])"))
		assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
	})
	cleanup(ctx, t)

	t.Run("multi-platform image", func(t *testing.T) {
		imgDataDir := t.TempDir()
		imgRef := "multiplatform:latest"
		_, mfstDescs, err := specialimage.MultiPlatform(imgDataDir, imgRef, []ocispec.Platform{linuxAmd64, linuxArm64, linuxRiscv64})
		assert.NilError(t, err)

		t.Run("one platform in index", func(t *testing.T) {
			platforms := []ocispec.Platform{linuxAmd64}
			err = tryLoad(ctx, t, imgDataDir, platforms)
			assert.NilError(t, err)

			// verify that the loaded image has the correct platform
			err = verifyImagePlatforms(ctx, imgSvc, imgRef, platforms)
			assert.NilError(t, err)
		})
		cleanup(ctx, t)

		t.Run("all platforms in index", func(t *testing.T) {
			platforms := []ocispec.Platform{linuxAmd64, linuxArm64, linuxRiscv64}
			err = tryLoad(ctx, t, imgDataDir, platforms)
			assert.NilError(t, err)

			// verify that the loaded image has the correct platforms
			err = verifyImagePlatforms(ctx, imgSvc, imgRef, platforms)
			assert.NilError(t, err)
		})
		cleanup(ctx, t)

		t.Run("platform not included in index", func(t *testing.T) {
			err = tryLoad(ctx, t, imgDataDir, []ocispec.Platform{linuxArmv5})
			assert.Check(t, is.Error(err, "image multiplatform:latest was loaded, but doesn't provide the requested platform ([linux/arm/v5])"))
			assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
		})
		cleanup(ctx, t)

		t.Run("platform included but blobs missing", func(t *testing.T) {
			// Assumption: arm64 image is second in the index (implementation detail of specialimage.MultiPlatform)
			mfstDesc := mfstDescs[1]
			assert.Assert(t, mfstDesc.Platform.Architecture == linuxArm64.Architecture)
			assert.Assert(t, mfstDesc.Platform.Variant == linuxArm64.Variant)

			t.Log(mfstDesc.Digest)

			// Delete arm64 manifest
			mfstPath := filepath.Join(imgDataDir, "blobs/sha256", mfstDesc.Digest.Encoded())
			assert.NilError(t, os.Remove(mfstPath))

			err = tryLoad(ctx, t, imgDataDir, []ocispec.Platform{linuxArm64})
			assert.Check(t, is.ErrorContains(err, "requested platform(s) ([linux/arm64]) found, but some content is missing"))
			assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
		})
		cleanup(ctx, t)
	})
}

func verifyImagePlatforms(ctx context.Context, imgSvc *ImageService, imgRef string, expectedPlatforms []ocispec.Platform) error {
	// get the manifest(s) for the image
	img, err := imgSvc.ImageInspect(ctx, imgRef, imagebackend.ImageInspectOpts{Manifests: true})
	if err != nil {
		return err
	}
	// verify that the image manifest has the expected platforms
	for _, ep := range expectedPlatforms {
		want := platforms.FormatAll(ep)
		found := false
		for _, m := range img.Manifests {
			if m.Descriptor.Platform != nil {
				got := platforms.FormatAll(*m.Descriptor.Platform)
				if got == want {
					found = true
					break
				}
			}
		}
		if !found {
			return fmt.Errorf("expected platform %q not found in loaded images", want)
		}
	}

	return nil
}

func TestImageLoadDanglingReferences(t *testing.T) {
	linuxAmd64 := ocispec.Platform{OS: "linux", Architecture: "amd64"}
	baseName := t.Name()

	newImageDir := func(name, content string) string {
		t.Helper()
		dir := t.TempDir()
		_, err := specialimage.MultiLayerCustom(dir, name, []specialimage.SingleFileLayer{
			{Name: "foo", Content: []byte(content)},
		})
		assert.NilError(t, err)
		return dir
	}

	newEnv := func(t *testing.T) *loadTestEnv {
		t.Helper()
		ctx := namespaces.WithNamespace(t.Context(), "testing-"+baseName)

		store, err := local.NewLabeledStore(t.TempDir(), &labelstore.InMemory{})
		assert.NilError(t, err)

		imgSvc := fakeImageService(t, ctx, store)
		imgSvc.defaultPlatformOverride = &linuxAmd64

		return &loadTestEnv{
			tryLoad: func(t *testing.T, dir string, platformList []ocispec.Platform) error {
				t.Helper()
				tarRc, err := archive.Tar(dir, compression.None)
				assert.NilError(t, err)
				defer tarRc.Close()

				buf := bytes.Buffer{}
				defer func() {
					t.Log(buf.String())
				}()

				return imgSvc.LoadImage(ctx, tarRc, platformList, &buf, true)
			},
			images: func(t *testing.T) []c8dimages.Image {
				t.Helper()
				imgs, err := imgSvc.images.List(ctx)
				assert.NilError(t, err)
				return imgs
			},
		}
	}

	t.Run("same tag and digest", func(t *testing.T) {
		env := newEnv(t)
		dir := newImageDir("same:latest", "1")

		for range 3 {
			assert.NilError(t, env.tryLoad(t, dir, nil))
		}

		imgs := env.images(t)
		assert.Assert(t, is.Len(imgs, 1), "expected a single named image, got %d", len(imgs))
		assert.Assert(t, is.Len(danglingImagesOf(imgs), 0), "no dangling duplicate expected")
	})

	t.Run("same tag different digest", func(t *testing.T) {
		env := newEnv(t)
		oldDir := newImageDir("replaced:latest", "1")
		assert.NilError(t, env.tryLoad(t, oldDir, nil))
		oldTarget := imageTarget(t, env, "docker.io/library/replaced:latest")

		newDir := newImageDir("replaced:latest", "2")
		assert.NilError(t, env.tryLoad(t, newDir, nil))
		newTarget := imageTarget(t, env, "docker.io/library/replaced:latest")
		assert.Assert(t, oldTarget != newTarget, "test images must have different targets")

		imgs := env.images(t)
		assert.Assert(t, is.Len(imgs, 2), "expected the named image plus the replaced dangling image, got %d", len(imgs))

		dangling := danglingImagesOf(imgs)
		assert.Assert(t, is.Len(dangling, 1), "expected the replaced target to stay dangling")
		assert.Assert(t, is.Equal(dangling[0].Target.Digest, oldTarget))
		for _, img := range imgs {
			if !isDanglingImage(img) {
				assert.Assert(t, is.Equal(img.Target.Digest, newTarget), "the tag must point to the new image")
			}
		}
	})

	t.Run("new tag", func(t *testing.T) {
		env := newEnv(t)
		dir := newImageDir("brand-new:latest", "1")
		assert.NilError(t, env.tryLoad(t, dir, nil))

		imgs := env.images(t)
		assert.Assert(t, is.Len(imgs, 1), "expected a single named image, got %d", len(imgs))
		assert.Assert(t, is.Len(danglingImagesOf(imgs), 0))
	})

	t.Run("multi-platform", func(t *testing.T) {
		env := newEnv(t)
		dir := t.TempDir()
		_, _, err := specialimage.MultiPlatform(dir, "multiplatform:latest", []ocispec.Platform{
			{OS: "linux", Architecture: "amd64"},
			{OS: "linux", Architecture: "arm64"},
		})
		assert.NilError(t, err)

		assert.NilError(t, env.tryLoad(t, dir, nil))
		firstLoad := env.images(t)

		assert.NilError(t, env.tryLoad(t, dir, nil))
		imgs := env.images(t)

		assert.Assert(t, is.Len(imgs, len(firstLoad)), "reloading must not add records (got %d -> %d)", len(firstLoad), len(imgs))
		assert.Assert(t, is.Len(danglingImagesOf(imgs), 0))
	})
}

type loadTestEnv struct {
	tryLoad func(t *testing.T, dir string, platformList []ocispec.Platform) error
	images  func(t *testing.T) []c8dimages.Image
}

func danglingImagesOf(imgs []c8dimages.Image) []c8dimages.Image {
	var dangling []c8dimages.Image
	for _, img := range imgs {
		if isDanglingImage(img) {
			dangling = append(dangling, img)
		}
	}
	return dangling
}

func imageTarget(t *testing.T, env *loadTestEnv, name string) digest.Digest {
	t.Helper()
	for _, img := range env.images(t) {
		if img.Name == name {
			return img.Target.Digest
		}
	}
	t.Fatalf("image %s not found in the store", name)
	return ""
}

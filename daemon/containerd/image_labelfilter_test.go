package containerd

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	c8dimages "github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/log/logtest"
	"github.com/containerd/platforms"
	"github.com/moby/moby/v2/daemon/internal/filters"
	"github.com/moby/moby/v2/internal/testutil/specialimage"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
)

// TestSetupLabelFilterNegativeValue is a regression test for the bug where
// --filter=label!=key=value did not match images that are missing the label
// entirely. Images without the label should satisfy the negated filter because
// they certainly do not have label=value. The old code returned false (no
// match) whenever the label was absent, regardless of whether the check was
// negated.
func TestSetupLabelFilterNegativeValue(t *testing.T) {
	ctx := namespaces.WithNamespace(t.Context(), "testing-"+t.Name())
	ctx = logtest.WithT(ctx, t)

	blobsDir := t.TempDir()
	cs := &blobsDirContentStore{blobs: filepath.Join(blobsDir, "blobs/sha256")}

	//
	// Build an image whose config carries the label on_prune=keep.
	//
	idxWithLabel, err := specialimage.MultiLayerCustom(blobsDir, "withlabel:latest", []specialimage.SingleFileLayer{
		{Name: "a", Content: []byte("layer-with-label")},
	})
	assert.NilError(t, err)

	imgWithLabel := imagesFromIndex(idxWithLabel)[0]

	// Patch the image: rewrite the config blob to include the label, then
	// rewrite the manifest to point to the new config blob.
	imgWithLabel = patchImageConfigLabels(t, ctx, cs, blobsDir, imgWithLabel, map[string]string{
		"on_prune": "keep",
	})

	//
	// Build an image without any custom labels.
	//
	idxNoLabel, err := specialimage.MultiLayerCustom(blobsDir, "nolabel:latest", []specialimage.SingleFileLayer{
		{Name: "b", Content: []byte("layer-no-label")},
	})
	assert.NilError(t, err)
	imgNoLabel := imagesFromIndex(idxNoLabel)[0]

	// Register both images with the fake image service.
	service := fakeImageService(t, ctx, cs)
	_, err = service.images.Create(ctx, imgWithLabel)
	assert.NilError(t, err)
	_, err = service.images.Create(ctx, imgNoLabel)
	assert.NilError(t, err)

	t.Run("label!=on_prune=keep includes image without label", func(t *testing.T) {
		// Images that do NOT have on_prune=keep should match this filter.
		fltrs := filters.NewArgs(filters.Arg("label!", "on_prune=keep"))
		fn, err := setupLabelFilter(ctx, cs, fltrs)
		assert.NilError(t, err)
		assert.Assert(t, fn != nil)

		// Image without the label: must match (true = include / prune).
		assert.Check(t, fn(imgNoLabel),
			"image without on_prune label should match filter label!=on_prune=keep")

		// Image with on_prune=keep: must NOT match.
		assert.Check(t, !fn(imgWithLabel),
			"image with on_prune=keep should not match filter label!=on_prune=keep")
	})

	t.Run("label=on_prune=keep includes only image with label", func(t *testing.T) {
		fltrs := filters.NewArgs(filters.Arg("label", "on_prune=keep"))
		fn, err := setupLabelFilter(ctx, cs, fltrs)
		assert.NilError(t, err)
		assert.Assert(t, fn != nil)

		assert.Check(t, !fn(imgNoLabel),
			"image without label should not match filter label=on_prune=keep")
		assert.Check(t, fn(imgWithLabel),
			"image with on_prune=keep should match filter label=on_prune=keep")
	})

	t.Run("label!=on_prune=other matches both images (neither has on_prune=other)", func(t *testing.T) {
		fltrs := filters.NewArgs(filters.Arg("label!", "on_prune=other"))
		fn, err := setupLabelFilter(ctx, cs, fltrs)
		assert.NilError(t, err)
		assert.Assert(t, fn != nil)

		// Neither image has on_prune=other, so both must match.
		assert.Check(t, fn(imgNoLabel),
			"image without label should match label!=on_prune=other")
		assert.Check(t, fn(imgWithLabel),
			"image with on_prune=keep should match label!=on_prune=other (value differs)")
	})
}

// patchImageConfigLabels rewrites the config blob of img so that it contains
// the provided labels, then rewrites the manifest blob to reference the new
// config, and returns an updated c8dimages.Image pointing to the new manifest.
func patchImageConfigLabels(t *testing.T, ctx context.Context, cs *blobsDirContentStore, blobsDir string, img c8dimages.Image, lbls map[string]string) c8dimages.Image {
	t.Helper()

	// Read existing manifest.
	var mfst ocispec.Manifest
	assert.NilError(t, readJSON(ctx, cs, img.Target, &mfst))

	// Read existing config and inject the labels.
	var cfg ocispec.Image
	cfg.Platform = platforms.DefaultSpec()
	assert.NilError(t, readJSON(ctx, cs, mfst.Config, &cfg))
	cfg.Config.Labels = lbls

	// Write the patched config blob.
	newConfigDesc := writeBlobToDir(t, blobsDir, mfst.Config.MediaType, cfg)

	// Write the patched manifest.
	mfst.Config = newConfigDesc
	newMfstDesc := writeBlobToDir(t, blobsDir, ocispec.MediaTypeImageManifest, mfst)
	newMfstDesc.Annotations = img.Target.Annotations

	img.Target = newMfstDesc
	return img
}

// writeBlobToDir serialises obj as JSON, writes it into blobsDir/blobs/sha256/,
// and returns an OCI descriptor for the blob.
func writeBlobToDir(t *testing.T, blobsDir string, mediaType string, obj any) ocispec.Descriptor {
	t.Helper()

	data, err := json.Marshal(obj)
	assert.NilError(t, err)

	dgst := digest.FromBytes(data)
	blobsPath := filepath.Join(blobsDir, "blobs", "sha256")
	assert.NilError(t, os.MkdirAll(blobsPath, 0o755))

	p := filepath.Join(blobsPath, dgst.Encoded())
	assert.NilError(t, os.WriteFile(p, data, 0o644))

	return ocispec.Descriptor{
		MediaType: mediaType,
		Digest:    dgst,
		Size:      int64(len(data)),
	}
}

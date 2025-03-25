package image // import "github.com/docker/docker/integration/image"

import (
	"slices"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/internal/testutils/specialimage"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestLoadDanglingImages(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	ctx := setupTest(t)

	client := testEnv.APIClient()

	specialimage.Load(ctx, t, client, func(dir string) (*ocispec.Index, error) {
		return specialimage.MultiLayerCustom(dir, "namedimage:latest", []specialimage.SingleFileLayer{
			{Name: "bar", Content: []byte("1")},
		})
	})

	// Should be one image.
	images, err := client.ImageList(ctx, image.ListOptions{})
	assert.NilError(t, err)

	findImageByName := func(images []image.Summary, imageName string) (image.Summary, error) {
		index := slices.IndexFunc(images, func(img image.Summary) bool {
			return slices.Index(img.RepoTags, imageName) >= 0
		})
		if index < 0 {
			return image.Summary{}, cerrdefs.ErrNotFound
		}
		return images[index], nil
	}

	oldImage, err := findImageByName(images, "namedimage:latest")
	assert.NilError(t, err)

	// Retain a copy of the old image and then replace it with a new one.
	specialimage.Load(ctx, t, client, func(dir string) (*ocispec.Index, error) {
		return specialimage.MultiLayerCustom(dir, "namedimage:latest", []specialimage.SingleFileLayer{
			{Name: "bar", Content: []byte("2")},
		})
	})

	images, err = client.ImageList(ctx, image.ListOptions{})
	assert.NilError(t, err)

	newImage, err := findImageByName(images, "namedimage:latest")
	assert.NilError(t, err)

	// IDs should be different.
	assert.Check(t, oldImage.ID != newImage.ID)

	// Should be able to find the original digest.
	findImageById := func(images []image.Summary, imageId string) (image.Summary, error) {
		index := slices.IndexFunc(images, func(img image.Summary) bool {
			return img.ID == imageId
		})
		if index < 0 {
			return image.Summary{}, cerrdefs.ErrNotFound
		}
		return images[index], nil
	}

	danglingImage, err := findImageById(images, oldImage.ID)
	assert.NilError(t, err)
	assert.Check(t, is.Len(danglingImage.RepoTags, 0))
}

package image

import (
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/integration/internal/build"
	iimage "github.com/docker/docker/integration/internal/image"
	"github.com/docker/docker/internal/testutils/specialimage"
	"github.com/docker/docker/testutil/fakecontext"
	"github.com/docker/docker/testutil/request"
	"github.com/moby/moby/api/types/image"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestLoadDanglingImages(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	ctx := setupTest(t)

	client := testEnv.APIClient()

	iimage.Load(ctx, t, client, func(dir string) (*ocispec.Index, error) {
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
	iimage.Load(ctx, t, client, func(dir string) (*ocispec.Index, error) {
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

func TestAPIImagesSaveAndLoad(t *testing.T) {
	ctx := setupTest(t)
	client := testEnv.APIClient()

	dockerfile := "FROM busybox\nENV FOO bar"

	imgID := build.Do(ctx, t, client, fakecontext.New(t, t.TempDir(), fakecontext.WithDockerfile(dockerfile)))

	res, body, err := request.Get(ctx, "/images/"+imgID+"/get")
	assert.NilError(t, err)
	defer body.Close()
	assert.Equal(t, res.StatusCode, http.StatusOK)

	res, loadBody, err := request.Post(ctx, "/images/load", request.RawContent(body), request.ContentType("application/x-tar"))
	assert.NilError(t, err)
	defer loadBody.Close()
	assert.Equal(t, res.StatusCode, http.StatusOK)

	loadBodyBytes, err := io.ReadAll(loadBody)
	assert.NilError(t, err)

	loadBodyContent := string(loadBodyBytes)
	assert.Assert(t, strings.Contains(loadBodyContent, imgID))
}

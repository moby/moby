package image

import (
	"testing"

	"github.com/docker/docker/testutil"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/client"

	"gotest.tools/v3/assert"
)

// This test checks to make sure the minimum supported client v1.24 against daemon gets correct Size.
func TestAPIImagesSizeCompatibility(t *testing.T) {
	ctx := setupTest(t)
	apiclient := testEnv.APIClient()
	defer apiclient.Close()

	images, err := apiclient.ImageList(ctx, image.ListOptions{})
	assert.NilError(t, err)
	assert.Assert(t, len(images) != 0)
	for _, img := range images {
		assert.Assert(t, img.Size != int64(-1))
	}

	apiclient, err = client.NewClientWithOpts(client.FromEnv, client.WithVersion("v1.24"))
	assert.NilError(t, err)
	defer apiclient.Close()

	v124Images, err := apiclient.ImageList(testutil.GetContext(t), image.ListOptions{})

	assert.NilError(t, err)
	assert.Assert(t, len(v124Images) != 0)
	for _, img := range v124Images {
		assert.Assert(t, img.Size != int64(-1))
	}
}

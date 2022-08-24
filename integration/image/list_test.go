package image // import "github.com/docker/docker/integration/image"

import (
	"context"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/versions"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

// Regression : #38171
func TestImagesFilterMultiReference(t *testing.T) {
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.40"), "broken in earlier versions")
	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	name := strings.ToLower(t.Name())
	repoTags := []string{
		name + ":v1",
		name + ":v2",
		name + ":v3",
		name + ":v4",
	}

	for _, repoTag := range repoTags {
		err := client.ImageTag(ctx, "busybox:latest", repoTag)
		assert.NilError(t, err)
	}

	filter := filters.NewArgs()
	filter.Add("reference", repoTags[0])
	filter.Add("reference", repoTags[1])
	filter.Add("reference", repoTags[2])
	options := types.ImageListOptions{
		All:     false,
		Filters: filter,
	}
	images, err := client.ImageList(ctx, options)
	assert.NilError(t, err)

	for _, img := range images {
		t.Logf("Image ID = %v, RepoTags = %+v", img.ID, img.RepoTags)
	}
	for _, tag := range filter.Get("reference") {
		assert.Check(t, is.Contains(images[0].RepoTags, tag))
	}
}

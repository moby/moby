package image // import "github.com/docker/docker/integration/image"

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/integration/internal/container"
	"github.com/google/go-cmp/cmp/cmpopts"
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

	assert.Check(t, is.Equal(len(images[0].RepoTags), 3))
	for _, repoTag := range images[0].RepoTags {
		if repoTag != repoTags[0] && repoTag != repoTags[1] && repoTag != repoTags[2] {
			t.Errorf("list images doesn't match any repoTag we expected, repoTag: %s", repoTag)
		}
	}
}

func TestImagesFilterBeforeSince(t *testing.T) {
	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	name := strings.ToLower(t.Name())
	ctr := container.Create(ctx, t, client, container.WithName(name))

	imgs := make([]string, 5)
	for i := range imgs {
		if i > 0 {
			// Make really really sure each image has a distinct timestamp.
			time.Sleep(time.Millisecond)
		}
		id, err := client.ContainerCommit(ctx, ctr, types.ContainerCommitOptions{Reference: fmt.Sprintf("%s:v%d", name, i)})
		assert.NilError(t, err)
		imgs[i] = id.ID
	}

	filter := filters.NewArgs(
		filters.Arg("since", imgs[0]),
		filters.Arg("before", imgs[len(imgs)-1]),
	)
	list, err := client.ImageList(ctx, types.ImageListOptions{Filters: filter})
	assert.NilError(t, err)

	var listedIDs []string
	for _, i := range list {
		t.Logf("ImageList: ID=%v RepoTags=%v", i.ID, i.RepoTags)
		listedIDs = append(listedIDs, i.ID)
	}
	// The ImageList API sorts the list by created timestamp... truncated to
	// 1-second precision. Since all the images were created within
	// milliseconds of each other, listedIDs is effectively unordered and
	// the assertion must therefore be order-independent.
	assert.DeepEqual(t, listedIDs, imgs[1:len(imgs)-1], cmpopts.SortSlices(func(a, b string) bool { return a < b }))
}

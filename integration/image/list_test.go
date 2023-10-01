package image // import "github.com/docker/docker/integration/image"

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/testutil"
	"github.com/google/go-cmp/cmp/cmpopts"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

// Regression : #38171
func TestImagesFilterMultiReference(t *testing.T) {
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.40"), "broken in earlier versions")
	ctx := setupTest(t)

	client := testEnv.APIClient()

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
		Filters: filter,
	}
	images, err := client.ImageList(ctx, options)
	assert.NilError(t, err)

	assert.Assert(t, is.Len(images, 1))
	assert.Check(t, is.Len(images[0].RepoTags, 3))
	for _, repoTag := range images[0].RepoTags {
		if repoTag != repoTags[0] && repoTag != repoTags[1] && repoTag != repoTags[2] {
			t.Errorf("list images doesn't match any repoTag we expected, repoTag: %s", repoTag)
		}
	}
}

func TestImagesFilterUntil(t *testing.T) {
	ctx := setupTest(t)

	client := testEnv.APIClient()

	name := strings.ToLower(t.Name())
	ctr := container.Create(ctx, t, client, container.WithName(name))

	imgs := make([]string, 5)
	for i := range imgs {
		if i > 0 {
			// Make really really sure each image has a distinct timestamp.
			time.Sleep(time.Millisecond)
		}
		id, err := client.ContainerCommit(ctx, ctr, containertypes.CommitOptions{Reference: fmt.Sprintf("%s:v%d", name, i)})
		assert.NilError(t, err)
		imgs[i] = id.ID
	}

	olderImage, _, err := client.ImageInspectWithRaw(ctx, imgs[2])
	assert.NilError(t, err)
	olderUntil := olderImage.Created

	laterImage, _, err := client.ImageInspectWithRaw(ctx, imgs[3])
	assert.NilError(t, err)
	laterUntil := laterImage.Created

	filter := filters.NewArgs(
		filters.Arg("since", imgs[0]),
		filters.Arg("until", olderUntil),
		filters.Arg("until", laterUntil),
		filters.Arg("before", imgs[len(imgs)-1]),
	)
	list, err := client.ImageList(ctx, types.ImageListOptions{Filters: filter})
	assert.NilError(t, err)

	var listedIDs []string
	for _, i := range list {
		t.Logf("ImageList: ID=%v RepoTags=%v", i.ID, i.RepoTags)
		listedIDs = append(listedIDs, i.ID)
	}
	assert.DeepEqual(t, listedIDs, imgs[1:2], cmpopts.SortSlices(func(a, b string) bool { return a < b }))
}

func TestImagesFilterBeforeSince(t *testing.T) {
	ctx := setupTest(t)

	client := testEnv.APIClient()

	name := strings.ToLower(t.Name())
	ctr := container.Create(ctx, t, client, container.WithName(name))

	imgs := make([]string, 5)
	for i := range imgs {
		if i > 0 {
			// Make really really sure each image has a distinct timestamp.
			time.Sleep(time.Millisecond)
		}
		id, err := client.ContainerCommit(ctx, ctr, containertypes.CommitOptions{Reference: fmt.Sprintf("%s:v%d", name, i)})
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

func TestAPIImagesFilters(t *testing.T) {
	ctx := setupTest(t)
	client := testEnv.APIClient()

	for _, n := range []string{"utest:tag1", "utest/docker:tag2", "utest:5000/docker:tag3"} {
		err := client.ImageTag(ctx, "busybox:latest", n)
		assert.NilError(t, err)
	}

	testcases := []struct {
		name             string
		filters          []filters.KeyValuePair
		expectedImages   int
		expectedRepoTags int
	}{
		{
			name:             "repository regex",
			filters:          []filters.KeyValuePair{filters.Arg("reference", "utest*/*")},
			expectedImages:   1,
			expectedRepoTags: 2,
		},
		{
			name:             "image name regex",
			filters:          []filters.KeyValuePair{filters.Arg("reference", "utest*")},
			expectedImages:   1,
			expectedRepoTags: 1,
		},
		{
			name:             "image name without a tag",
			filters:          []filters.KeyValuePair{filters.Arg("reference", "utest")},
			expectedImages:   1,
			expectedRepoTags: 1,
		},
		{
			name:             "registry port regex",
			filters:          []filters.KeyValuePair{filters.Arg("reference", "*5000*/*")},
			expectedImages:   1,
			expectedRepoTags: 1,
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := testutil.StartSpan(ctx, t)
			images, err := client.ImageList(ctx, types.ImageListOptions{
				Filters: filters.NewArgs(tc.filters...),
			})
			assert.Check(t, err)
			assert.Assert(t, is.Len(images, tc.expectedImages))
			assert.Check(t, is.Len(images[0].RepoTags, tc.expectedRepoTags))
		})
	}
}

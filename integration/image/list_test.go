package image // import "github.com/docker/docker/integration/image"

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/internal/testutils/specialimage"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/daemon"
	"github.com/google/go-cmp/cmp/cmpopts"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

// Regression : #38171
func TestImagesFilterMultiReference(t *testing.T) {
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
	options := image.ListOptions{
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
			// Make sure each image has a distinct timestamp.
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
	list, err := client.ImageList(ctx, image.ListOptions{Filters: filter})
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
			// Make sure each image has a distinct timestamp.
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
	list, err := client.ImageList(ctx, image.ListOptions{Filters: filter})
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
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := testutil.StartSpan(ctx, t)
			images, err := client.ImageList(ctx, image.ListOptions{
				Filters: filters.NewArgs(tc.filters...),
			})
			assert.Check(t, err)
			assert.Assert(t, is.Len(images, tc.expectedImages))
			assert.Check(t, is.Len(images[0].RepoTags, tc.expectedRepoTags))
		})
	}
}

// Verify that the size calculation operates on ChainIDs and not DiffIDs.
// This test calls an image list with two images that share one, top layer.
func TestAPIImagesListSizeShared(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	ctx := setupTest(t)

	daemon := daemon.New(t)
	daemon.Start(t)
	defer daemon.Stop(t)

	client := daemon.NewClientT(t)

	specialimage.Load(ctx, t, client, func(dir string) (*ocispec.Index, error) {
		return specialimage.MultiLayerCustom(dir, "multilayer:latest", []specialimage.SingleFileLayer{
			{Name: "bar", Content: []byte("2")},
			{Name: "foo", Content: []byte("1")},
		})
	})

	specialimage.Load(ctx, t, client, func(dir string) (*ocispec.Index, error) {
		return specialimage.MultiLayerCustom(dir, "multilayer2:latest", []specialimage.SingleFileLayer{
			{Name: "asdf", Content: []byte("3")},
			{Name: "foo", Content: []byte("1")},
		})
	})

	_, err := client.ImageList(ctx, image.ListOptions{SharedSize: true})
	assert.NilError(t, err)
}

func TestAPIImagesListManifests(t *testing.T) {
	skip.If(t, !testEnv.UsingSnapshotter())
	// Sub-daemons not supported on Windows
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

	ctx := setupTest(t)

	d := daemon.New(t)
	d.Start(t)
	defer d.Stop(t)

	apiClient := d.NewClientT(t)

	testPlatforms := []ocispec.Platform{
		{OS: "windows", Architecture: "amd64"},
		{OS: "linux", Architecture: "arm", Variant: "v7"},
		{OS: "darwin", Architecture: "arm64"},
	}
	specialimage.Load(ctx, t, apiClient, func(dir string) (*ocispec.Index, error) {
		idx, _, err := specialimage.MultiPlatform(dir, "multiplatform:latest", testPlatforms)
		return idx, err
	})

	containerPlatform := testPlatforms[1]

	cid := container.Create(ctx, t, apiClient,
		container.WithImage("multiplatform:latest"),
		container.WithPlatform(&containerPlatform))

	t.Run("unsupported before 1.47", func(t *testing.T) {
		// TODO: Remove when MinSupportedAPIVersion >= 1.47
		c := d.NewClientT(t, client.WithVersion(api.MinSupportedAPIVersion))

		images, err := c.ImageList(ctx, image.ListOptions{Manifests: true})
		assert.NilError(t, err)

		assert.Assert(t, is.Len(images, 1))
		assert.Check(t, is.Nil(images[0].Manifests))
	})

	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.47"))

	api147 := d.NewClientT(t, client.WithVersion("1.47"))

	t.Run("no manifests if not requested", func(t *testing.T) {
		images, err := api147.ImageList(ctx, image.ListOptions{})
		assert.NilError(t, err)

		assert.Assert(t, is.Len(images, 1))
		assert.Check(t, is.Nil(images[0].Manifests))
	})

	images, err := api147.ImageList(ctx, image.ListOptions{Manifests: true})
	assert.NilError(t, err)

	assert.Check(t, is.Len(images, 1))
	assert.Check(t, images[0].Manifests != nil)
	assert.Check(t, is.Len(images[0].Manifests, 3))

	for _, mfst := range images[0].Manifests {
		// All manifests should be image manifests
		assert.Check(t, is.Equal(mfst.Kind, image.ManifestKindImage))

		// Full image was loaded so all manifests should be available
		assert.Check(t, mfst.Available)

		// The platform should be one of the test platforms
		if assert.Check(t, is.Contains(testPlatforms, mfst.ImageData.Platform)) {
			testPlatforms = slices.DeleteFunc(testPlatforms, func(p ocispec.Platform) bool {
				op := mfst.ImageData.Platform
				return p.OS == op.OS && p.Architecture == op.Architecture && p.Variant == op.Variant
			})
		}

		if mfst.ImageData.Platform.OS == containerPlatform.OS &&
			mfst.ImageData.Platform.Architecture == containerPlatform.Architecture &&
			mfst.ImageData.Platform.Variant == containerPlatform.Variant {

			assert.Check(t, is.DeepEqual(mfst.ImageData.Containers, []string{cid}))
		}
	}
}

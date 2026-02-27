package image

import (
	"context"
	"io"
	"testing"

	buildtypes "github.com/moby/moby/api/types/build"
	"github.com/moby/moby/client"
	build "github.com/moby/moby/v2/integration/internal/build"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/fakecontext"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestAPIImagesHistory(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	dockerfile := "FROM busybox\nENV FOO bar"

	imgID := build.Do(ctx, t, apiClient, fakecontext.New(t, t.TempDir(), fakecontext.WithDockerfile(dockerfile)))

	res, err := apiClient.ImageHistory(ctx, imgID)
	assert.NilError(t, err)

	assert.Assert(t, len(res.Items) != 0)

	var found bool
	for _, imageLayer := range res.Items {
		if imageLayer.ID == imgID {
			found = true
			break
		}
	}

	assert.Assert(t, found)
}

// TestAPIImageHistoryCrossPlatform tests the image history functionality
// when dealing with cross-platform image builds.
// This is a regression test for https://github.com/moby/moby/issues/50851
// where `docker history` fails with "snapshot does not exist" error for
// images built for non-native platforms.
func TestAPIImageHistoryCrossPlatform(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	// Determine the non-native platform to use for testing
	nonNativePlatform := ocispec.Platform{OS: testEnv.DaemonInfo.OSType, Architecture: "amd64"}
	if testEnv.DaemonInfo.Architecture == "amd64" {
		nonNativePlatform = ocispec.Platform{OS: testEnv.DaemonInfo.OSType, Architecture: "arm64"}
	}

	// We need to pull the image for the non-native platform
	// TODO: Make sure we have a multi-platform frozen image we could use
	pullImageForPlatform(t, ctx, apiClient, "alpine", nonNativePlatform)

	dockerfile := "FROM alpine\nRUN true"

	buildCtx := fakecontext.New(t, t.TempDir(), fakecontext.WithDockerfile(dockerfile))

	// Build the image for a non-native platform
	resp, err := apiClient.ImageBuild(ctx, buildCtx.AsTarReader(t), client.ImageBuildOptions{
		Version:   buildtypes.BuilderBuildKit,
		Tags:      []string{"cross-platform-test"},
		Platforms: []ocispec.Platform{nonNativePlatform},
	})
	assert.NilError(t, err)
	defer resp.Body.Close()

	imgID := build.GetImageIDFromBody(t, resp.Body)
	t.Cleanup(func() {
		_, _ = apiClient.ImageRemove(ctx, imgID, client.ImageRemoveOptions{Force: true})
	})

	testCases := []struct {
		name     string
		imageRef string
		options  []client.ImageHistoryOption
	}{
		{
			name:     "without explicit platform",
			imageRef: imgID,
			options:  nil,
		},
		{
			name:     "with explicit platform",
			imageRef: imgID,
			options:  []client.ImageHistoryOption{client.ImageHistoryWithPlatform(nonNativePlatform)},
		},
		{
			name:     "using image reference",
			imageRef: "cross-platform-test",
			options:  nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)

			res, err := apiClient.ImageHistory(ctx, tc.imageRef, tc.options...)

			assert.NilError(t, err)
			found := false
			for _, layer := range res.Items {
				if layer.ID == imgID {
					found = true
					break
				}
			}
			assert.Assert(t, found, "History should contain the built image ID")
			assert.Assert(t, is.Len(res.Items, 3))

			for i, layer := range res.Items {
				assert.Assert(t, layer.Size >= 0, "Layer %d should not have negative size", i)
			}
		})
	}
}

func pullImageForPlatform(t *testing.T, ctx context.Context, apiClient client.APIClient, ref string, platform ocispec.Platform) {
	pullResp, err := apiClient.ImagePull(ctx, ref, client.ImagePullOptions{
		Platforms: []ocispec.Platform{platform},
	})
	assert.NilError(t, err)
	_, _ = io.Copy(io.Discard, pullResp)

	_, err = apiClient.ImageInspect(ctx, ref)
	assert.NilError(t, err)

	t.Cleanup(func() {
		_, _ = apiClient.ImageRemove(ctx, ref, client.ImageRemoveOptions{Force: true})
	})
}

package image

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/internal/testutils/specialimage"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

// Regression test for: https://github.com/moby/moby/issues/45556
func TestImageInspectEmptyTagsAndDigests(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "build-empty-images is not called on Windows")
	ctx := setupTest(t)

	apiClient := testEnv.APIClient()

	danglingID := specialimage.Load(ctx, t, apiClient, specialimage.Dangling)

	var raw bytes.Buffer
	inspect, err := apiClient.ImageInspect(ctx, danglingID, client.ImageInspectWithRawResponse(&raw))
	assert.NilError(t, err)

	// Must be a zero length array, not null.
	assert.Check(t, is.Len(inspect.RepoTags, 0))
	assert.Check(t, is.Len(inspect.RepoDigests, 0))

	var rawJson map[string]interface{}
	err = json.Unmarshal(raw.Bytes(), &rawJson)
	assert.NilError(t, err)

	// Check if the raw json is also an array, not null.
	assert.Check(t, is.Len(rawJson["RepoTags"], 0))
	assert.Check(t, is.Len(rawJson["RepoDigests"], 0))
}

// Regression test for: https://github.com/moby/moby/issues/48747
func TestImageInspectUniqueRepoDigests(t *testing.T) {
	ctx := setupTest(t)

	client := testEnv.APIClient()

	before, err := client.ImageInspect(ctx, "busybox")
	assert.NilError(t, err)

	for _, tag := range []string{"master", "newest"} {
		imgName := "busybox:" + tag
		err := client.ImageTag(ctx, "busybox", imgName)
		assert.NilError(t, err)
		defer func() {
			_, _ = client.ImageRemove(ctx, imgName, image.RemoveOptions{Force: true})
		}()
	}

	after, err := client.ImageInspect(ctx, "busybox")
	assert.NilError(t, err)

	assert.Check(t, is.Len(after.RepoDigests, len(before.RepoDigests)))
}

func TestImageInspectDescriptor(t *testing.T) {
	ctx := setupTest(t)

	client := testEnv.APIClient()

	inspect, err := client.ImageInspect(ctx, "busybox")
	assert.NilError(t, err)

	if !testEnv.UsingSnapshotter() {
		assert.Check(t, is.Nil(inspect.Descriptor))
		return
	}

	assert.Assert(t, inspect.Descriptor != nil)
	assert.Check(t, inspect.Descriptor.Digest.String() == inspect.ID)
	assert.Check(t, inspect.Descriptor.Size > 0)
}

func TestImageInspectWithPlatform(t *testing.T) {
	ctx := setupTest(t)

	apiClient := testEnv.APIClient()

	nativePlatform := ocispec.Platform{
		OS:           testEnv.DaemonInfo.OSType,
		Architecture: testEnv.DaemonInfo.Architecture,
	}

	// Create a platform that does not match the host platform
	differentOS := "linux"
	if nativePlatform.OS == "linux" {
		differentOS = "windows"
	}
	differentPlatform := ocispec.Platform{
		OS:           differentOS,
		Architecture: "amd64",
	}

	imageID := specialimage.Load(ctx, t, apiClient, func(dir string) (*ocispec.Index, error) {
		i, _, err := specialimage.MultiPlatform(dir, "multiplatform:latest", []ocispec.Platform{nativePlatform, differentPlatform})
		return i, err
	})

	for _, tc := range []struct {
		name              string
		requestedPlatform *ocispec.Platform
		expectedPlatform  *ocispec.Platform
	}{
		{
			name:              "default",
			requestedPlatform: nil,
			expectedPlatform:  &nativePlatform,
		},
		{
			name:              "native",
			requestedPlatform: &nativePlatform,
			expectedPlatform:  &nativePlatform,
		},
		{
			name:              "different",
			requestedPlatform: &differentPlatform,
			expectedPlatform:  &differentPlatform,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var opts []client.ImageInspectOption
			if tc.requestedPlatform != nil {
				opts = append(opts, client.ImageInspectWithPlatform(tc.requestedPlatform))
				opts = append(opts, client.ImageInspectWithManifests(true))
			}
			inspect, err := apiClient.ImageInspect(ctx, imageID, opts...)
			assert.NilError(t, err)

			assert.Check(t, is.Equal(inspect.Architecture, nativePlatform.Architecture))
			assert.Check(t, is.Equal(inspect.Os, nativePlatform.OS))

			assert.Assert(t, inspect.Descriptor != nil)
			assert.Check(t, is.DeepEqual(*inspect.Descriptor.Platform, *tc.expectedPlatform))

			if tc.requestedPlatform != nil {
				t.Run("has no manifests", func(t *testing.T) {
					assert.Check(t, is.Nil(inspect.Manifests))
				})
			}
		})
	}
}

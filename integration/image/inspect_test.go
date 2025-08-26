package image

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/moby/moby/client"
	iimage "github.com/moby/moby/v2/integration/internal/image"
	"github.com/moby/moby/v2/internal/testutils/specialimage"
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

	danglingID := iimage.Load(ctx, t, apiClient, specialimage.Dangling)

	var raw bytes.Buffer
	inspect, err := apiClient.ImageInspect(ctx, danglingID, client.ImageInspectWithRawResponse(&raw))
	assert.NilError(t, err)

	// Must be a zero length array, not null.
	assert.Check(t, is.Len(inspect.RepoTags, 0))
	assert.Check(t, is.Len(inspect.RepoDigests, 0))

	var rawJson map[string]any
	err = json.Unmarshal(raw.Bytes(), &rawJson)
	assert.NilError(t, err)

	// Check if the raw json is also an array, not null.
	assert.Check(t, is.Len(rawJson["RepoTags"], 0))
	assert.Check(t, is.Len(rawJson["RepoDigests"], 0))
}

// Regression test for: https://github.com/moby/moby/issues/48747
func TestImageInspectUniqueRepoDigests(t *testing.T) {
	ctx := setupTest(t)

	apiClient := testEnv.APIClient()

	before, err := apiClient.ImageInspect(ctx, "busybox")
	assert.NilError(t, err)

	for _, tag := range []string{"master", "newest"} {
		imgName := "busybox:" + tag
		err := apiClient.ImageTag(ctx, "busybox", imgName)
		assert.NilError(t, err)
		defer func() {
			_, _ = apiClient.ImageRemove(ctx, imgName, client.ImageRemoveOptions{Force: true})
		}()
	}

	after, err := apiClient.ImageInspect(ctx, "busybox")
	assert.NilError(t, err)

	assert.Check(t, is.Len(after.RepoDigests, len(before.RepoDigests)))
}

func TestImageInspectDescriptor(t *testing.T) {
	ctx := setupTest(t)

	apiClient := testEnv.APIClient()

	inspect, err := apiClient.ImageInspect(ctx, "busybox")
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
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "The test image is a Linux image")
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

	imageID := iimage.Load(ctx, t, apiClient, func(dir string) (*ocispec.Index, error) {
		i, descs, err := specialimage.MultiPlatform(dir, "multiplatform:latest", []ocispec.Platform{nativePlatform, differentPlatform})
		assert.NilError(t, err)

		err = specialimage.LegacyManifest(dir, "multiplatform:latest", descs[0])
		assert.NilError(t, err)

		return i, err
	})

	for _, tc := range []struct {
		name              string
		requestedPlatform *ocispec.Platform
		expectedPlatform  *ocispec.Platform
		expectedError     string
		withManifests     bool
		snapshotterOnly   bool
		graphdriverOnly   bool
	}{
		{
			name:              "default",
			requestedPlatform: nil,
			expectedPlatform:  &nativePlatform,
		},
		{
			name:              "snapshotter/with-manifests",
			requestedPlatform: nil,
			expectedPlatform:  &nativePlatform,
			snapshotterOnly:   true,
			withManifests:     true,
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
			snapshotterOnly:   true,
		},
		{
			name:              "different not supported on graphdriver",
			requestedPlatform: &differentPlatform,
			graphdriverOnly:   true,
			//  image with reference multiplatform:latest was found but its platform (linux/aarch64) does not match the specified platform (windows/amd64)
			expectedError: "image with reference multiplatform:latest was found but its platform",
		},
	} {
		if tc.snapshotterOnly && !testEnv.UsingSnapshotter() {
			continue
		}
		if tc.graphdriverOnly && testEnv.UsingSnapshotter() {
			continue
		}

		t.Run(tc.name, func(t *testing.T) {
			var opts []client.ImageInspectOption
			if tc.requestedPlatform != nil {
				opts = append(opts, client.ImageInspectWithPlatform(tc.requestedPlatform))
			}
			if tc.withManifests {
				opts = append(opts, client.ImageInspectWithManifests(true))
			}
			inspect, err := apiClient.ImageInspect(ctx, imageID, opts...)
			if tc.expectedError != "" {
				assert.Assert(t, is.ErrorContains(err, tc.expectedError))
				return
			}
			assert.NilError(t, err)

			assert.Check(t, is.Equal(inspect.Architecture, tc.expectedPlatform.Architecture))
			assert.Check(t, is.Equal(inspect.Os, tc.expectedPlatform.OS))

			if testEnv.UsingSnapshotter() {
				assert.Assert(t, inspect.Descriptor != nil)
				if tc.requestedPlatform != nil {
					if assert.Check(t, inspect.Descriptor.Platform != nil) {
						assert.Check(t, is.DeepEqual(*inspect.Descriptor.Platform, *tc.expectedPlatform))
					}
				}
			} else {
				assert.Check(t, inspect.Descriptor == nil)
			}

			if tc.withManifests {
				t.Run("has manifests", func(t *testing.T) {
					assert.Check(t, is.Len(inspect.Manifests, 2))
				})
			} else {
				t.Run("has no manifests", func(t *testing.T) {
					assert.Check(t, is.Nil(inspect.Manifests))
				})
			}
		})
	}
}

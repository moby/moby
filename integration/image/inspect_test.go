package image

import (
	"encoding/json"
	"testing"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/internal/testutils/specialimage"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

// Regression test for: https://github.com/moby/moby/issues/45556
func TestImageInspectEmptyTagsAndDigests(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "build-empty-images is not called on Windows")
	ctx := setupTest(t)

	client := testEnv.APIClient()

	danglingID := specialimage.Load(ctx, t, client, specialimage.Dangling)

	inspect, raw, err := client.ImageInspectWithRaw(ctx, danglingID)
	assert.NilError(t, err)

	// Must be a zero length array, not null.
	assert.Check(t, is.Len(inspect.RepoTags, 0))
	assert.Check(t, is.Len(inspect.RepoDigests, 0))

	var rawJson map[string]interface{}
	err = json.Unmarshal(raw, &rawJson)
	assert.NilError(t, err)

	// Check if the raw json is also an array, not null.
	assert.Check(t, is.Len(rawJson["RepoTags"], 0))
	assert.Check(t, is.Len(rawJson["RepoDigests"], 0))
}

// Regression test for: https://github.com/moby/moby/issues/48747
func TestImageInspectUniqueRepoDigests(t *testing.T) {
	ctx := setupTest(t)

	client := testEnv.APIClient()

	before, _, err := client.ImageInspectWithRaw(ctx, "busybox")
	assert.NilError(t, err)

	for _, tag := range []string{"master", "newest"} {
		imgName := "busybox:" + tag
		err := client.ImageTag(ctx, "busybox", imgName)
		assert.NilError(t, err)
		defer func() {
			_, _ = client.ImageRemove(ctx, imgName, image.RemoveOptions{Force: true})
		}()
	}

	after, _, err := client.ImageInspectWithRaw(ctx, "busybox")
	assert.NilError(t, err)

	assert.Check(t, is.Len(after.RepoDigests, len(before.RepoDigests)))
}

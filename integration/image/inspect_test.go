package image

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/docker/docker/testutil/environment"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

// Regression test for: https://github.com/moby/moby/issues/45556
func TestImageInspectEmptyTagsAndDigests(t *testing.T) {
	skip.If(t, testEnv.OSType == "windows", "build-empty-images is not called on Windows")
	defer setupTest(t)()

	client := testEnv.APIClient()
	ctx := context.Background()

	danglingId := environment.DanglingImageIdGraphDriver
	if testEnv.UsingSnapshotter() {
		danglingId = environment.DanglingImageIdSnapshotter
	}

	inspect, raw, err := client.ImageInspectWithRaw(ctx, danglingId)
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

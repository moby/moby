package image

import (
	"testing"

	"github.com/moby/moby/v2/integration/internal/build"
	"github.com/moby/moby/v2/testutil/fakecontext"
	"gotest.tools/v3/assert"
)

func TestAPIImagesHistory(t *testing.T) {
	ctx := setupTest(t)
	client := testEnv.APIClient()

	dockerfile := "FROM busybox\nENV FOO bar"

	imgID := build.Do(ctx, t, client, fakecontext.New(t, t.TempDir(), fakecontext.WithDockerfile(dockerfile)))

	historydata, err := client.ImageHistory(ctx, imgID)
	assert.NilError(t, err)

	assert.Assert(t, len(historydata) != 0)

	var found bool
	for _, imageLayer := range historydata {
		if imageLayer.ID == imgID {
			found = true
			break
		}
	}

	assert.Assert(t, found)
}

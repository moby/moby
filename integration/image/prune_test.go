package image

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/testutil/environment"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

// Regression test for: https://github.com/moby/moby/issues/45732
func TestPruneDontDeleteUsedDangling(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "FIXME: hack/make/.build-empty-images doesn't run on Windows")

	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	danglingID := environment.GetTestDanglingImageId(testEnv)

	container.Create(ctx, t, client,
		container.WithImage(danglingID),
		container.WithCmd("sleep", "60"))

	pruned, err := client.ImagesPrune(ctx, filters.NewArgs(filters.Arg("dangling", "true")))

	assert.NilError(t, err)
	assert.Check(t, is.Len(pruned.ImagesDeleted, 0))
}

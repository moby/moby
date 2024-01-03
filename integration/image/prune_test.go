package image

import (
	"strings"
	"testing"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/internal/testutils/specialimage"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

// Regression test for: https://github.com/moby/moby/issues/45732
func TestPruneDontDeleteUsedDangling(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "cannot start multiple daemons on windows")
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")

	ctx := setupTest(t)

	d := daemon.New(t)
	d.Start(t)
	defer d.Stop(t)

	client := d.NewClientT(t)
	defer client.Close()

	danglingID := specialimage.Load(ctx, t, client, specialimage.Dangling)

	_, _, err := client.ImageInspectWithRaw(ctx, danglingID)
	assert.NilError(t, err, "Test dangling image doesn't exist")

	container.Create(ctx, t, client,
		container.WithImage(danglingID),
		container.WithCmd("sleep", "60"))

	pruned, err := client.ImagesPrune(ctx, filters.NewArgs(filters.Arg("dangling", "true")))
	assert.NilError(t, err)

	for _, deleted := range pruned.ImagesDeleted {
		if strings.Contains(deleted.Deleted, danglingID) || strings.Contains(deleted.Untagged, danglingID) {
			t.Errorf("used dangling image %s shouldn't be deleted", danglingID)
		}
	}

	_, _, err = client.ImageInspectWithRaw(ctx, danglingID)
	assert.NilError(t, err, "Test dangling image should still exist")
}

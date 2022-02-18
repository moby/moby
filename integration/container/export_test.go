package container // import "github.com/moby/moby/integration/container"

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types"
	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/integration/internal/container"
	"github.com/moby/moby/pkg/jsonmessage"
	"github.com/moby/moby/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

// export an image and try to import it into a new one
func TestExportContainerAndImportImage(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	cID := container.Run(ctx, t, client, container.WithCmd("true"))
	poll.WaitOn(t, container.IsStopped(ctx, client, cID), poll.WithDelay(100*time.Millisecond))

	reference := "repo/" + strings.ToLower(t.Name()) + ":v1"
	exportResp, err := client.ContainerExport(ctx, cID)
	assert.NilError(t, err)
	importResp, err := client.ImageImport(ctx, types.ImageImportSource{
		Source:     exportResp,
		SourceName: "-",
	}, reference, types.ImageImportOptions{})
	assert.NilError(t, err)

	// If the import is successfully, then the message output should contain
	// the image ID and match with the output from `docker images`.

	dec := json.NewDecoder(importResp)
	var jm jsonmessage.JSONMessage
	err = dec.Decode(&jm)
	assert.NilError(t, err)

	images, err := client.ImageList(ctx, types.ImageListOptions{
		Filters: filters.NewArgs(filters.Arg("reference", reference)),
	})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(jm.Status, images[0].ID))
}

// TestExportContainerAfterDaemonRestart checks that a container
// created before start of the currently running dockerd
// can be exported (as reported in #36561). To satisfy this
// condition, daemon restart is needed after container creation.
func TestExportContainerAfterDaemonRestart(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, testEnv.IsRemoteDaemon)

	d := daemon.New(t)
	c := d.NewClientT(t)

	d.StartWithBusybox(t)
	defer d.Stop(t)

	ctx := context.Background()
	ctrID := container.Create(ctx, t, c)

	d.Restart(t)

	_, err := c.ContainerExport(ctx, ctrID)
	assert.NilError(t, err)
}

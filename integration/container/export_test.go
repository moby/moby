package container // import "github.com/docker/docker/integration/container"

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

// export an image and try to import it into a new one
func TestExportContainerAndImportImage(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	cID := container.Run(ctx, t, apiClient, container.WithCmd("true"))
	poll.WaitOn(t, container.IsStopped(ctx, apiClient, cID))

	reference := "repo/" + strings.ToLower(t.Name()) + ":v1"
	exportResp, err := apiClient.ContainerExport(ctx, cID)
	assert.NilError(t, err)
	importResp, err := apiClient.ImageImport(ctx, image.ImportSource{
		Source:     exportResp,
		SourceName: "-",
	}, reference, image.ImportOptions{})
	assert.NilError(t, err)

	// If the import is successfully, then the message output should contain
	// the image ID and match with the output from `docker images`.

	dec := json.NewDecoder(importResp)
	var jm jsonmessage.JSONMessage
	err = dec.Decode(&jm)
	assert.NilError(t, err)

	images, err := apiClient.ImageList(ctx, image.ListOptions{
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

	ctx := testutil.StartSpan(baseContext, t)

	d := daemon.New(t)
	c := d.NewClientT(t)

	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)

	ctrID := container.Create(ctx, t, c)

	d.Restart(t)

	_, err := c.ContainerExport(ctx, ctrID)
	assert.NilError(t, err)
}

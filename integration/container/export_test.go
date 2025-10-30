package container

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/moby/moby/api/types/jsonstream"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/integration/internal/container"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/daemon"
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
	exportRes, err := apiClient.ContainerExport(ctx, cID, client.ContainerExportOptions{})
	assert.NilError(t, err)
	importRes, err := apiClient.ImageImport(ctx, client.ImageImportSource{
		Source:     exportRes.Body,
		SourceName: "-",
	}, reference, client.ImageImportOptions{})
	assert.NilError(t, err)
	defer func() { _ = importRes.Body.Close() }()

	// If the import is successfully, then the message output should contain
	// the image ID and match with the output from `docker images`.

	dec := json.NewDecoder(importRes.Body)
	var jm jsonstream.Message
	err = dec.Decode(&jm)
	assert.NilError(t, err)

	images, err := apiClient.ImageList(ctx, client.ImageListOptions{
		Filters: make(client.Filters).Add("reference", reference),
	})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(jm.Status, images.Items[0].ID))
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

	res, err := c.ContainerExport(ctx, ctrID, client.ContainerExportOptions{})
	assert.NilError(t, err)
	_ = res.Body.Close()
}

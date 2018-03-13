package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	containerTypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/integration-cli/daemon"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/request"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/gotestyourself/gotestyourself/assert"
	is "github.com/gotestyourself/gotestyourself/assert/cmp"
	"github.com/gotestyourself/gotestyourself/poll"
	"github.com/gotestyourself/gotestyourself/skip"
)

// export an image and try to import it into a new one
func TestExportContainerAndImportImage(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	cID := container.Run(t, ctx, client, container.WithCmd("true"))
	poll.WaitOn(t, container.IsStopped(ctx, client, cID), poll.WithDelay(100*time.Millisecond))

	reference := "repo/testexp:v1"
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
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, testEnv.IsRemoteDaemon())

	d := daemon.New(t, "", "dockerd", daemon.Config{})
	client, err := d.NewClient()
	assert.NilError(t, err)

	d.StartWithBusybox(t)
	defer d.Stop(t)

	ctx := context.Background()
	cfg := containerTypes.Config{
		Image: "busybox",
		Cmd:   []string{"top"},
	}
	ctr, err := client.ContainerCreate(ctx, &cfg, nil, nil, "")
	assert.NilError(t, err)

	d.Restart(t)

	_, err = client.ContainerExport(ctx, ctr.ID)
	assert.NilError(t, err)
}

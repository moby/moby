package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/integration/internal/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/fs"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func getPrefixAndSlashFromDaemonPlatform() (prefix, slash string) {
	if testEnv.DaemonInfo.OSType == "windows" {
		return "c:", `\`
	}
	return "", "/"
}

// Test case for #5244: `docker rm` fails if bind dir doesn't exist anymore
func TestRemoveContainerWithRemovedVolume(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)

	defer setupTest(t)()
	ctx := context.Background()
	client := testEnv.APIClient()

	prefix, slash := getPrefixAndSlashFromDaemonPlatform()

	tempDir := fs.NewDir(t, "test-rm-container-with-removed-volume", fs.WithMode(0755))
	defer tempDir.Remove()

	cID := container.Run(ctx, t, client, container.WithCmd("true"), container.WithBind(tempDir.Path(), prefix+slash+"test"))
	poll.WaitOn(t, container.IsInState(ctx, client, cID, "exited"), poll.WithDelay(100*time.Millisecond))

	err := os.RemoveAll(tempDir.Path())
	assert.NilError(t, err)

	err = client.ContainerRemove(ctx, cID, types.ContainerRemoveOptions{
		RemoveVolumes: true,
	})
	assert.NilError(t, err)

	_, _, err = client.ContainerInspectWithRaw(ctx, cID, true)
	assert.Check(t, is.ErrorContains(err, "No such container"))
}

// Test case for #2099/#2125
func TestRemoveContainerWithVolume(t *testing.T) {
	defer setupTest(t)()
	ctx := context.Background()
	client := testEnv.APIClient()

	prefix, slash := getPrefixAndSlashFromDaemonPlatform()

	cID := container.Run(ctx, t, client, container.WithCmd("true"), container.WithVolume(prefix+slash+"srv"))
	poll.WaitOn(t, container.IsInState(ctx, client, cID, "exited"), poll.WithDelay(100*time.Millisecond))

	insp, _, err := client.ContainerInspectWithRaw(ctx, cID, true)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(1, len(insp.Mounts)))
	volName := insp.Mounts[0].Name

	err = client.ContainerRemove(ctx, cID, types.ContainerRemoveOptions{
		RemoveVolumes: true,
	})
	assert.NilError(t, err)

	volumes, err := client.VolumeList(ctx, volume.ListOptions{
		Filters: filters.NewArgs(filters.Arg("name", volName)),
	})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(0, len(volumes.Volumes)))
}

func TestRemoveContainerRunning(t *testing.T) {
	defer setupTest(t)()
	ctx := context.Background()
	client := testEnv.APIClient()

	cID := container.Run(ctx, t, client)

	err := client.ContainerRemove(ctx, cID, types.ContainerRemoveOptions{})
	assert.Check(t, is.ErrorContains(err, "cannot remove a running container"))
}

func TestRemoveContainerForceRemoveRunning(t *testing.T) {
	defer setupTest(t)()
	ctx := context.Background()
	client := testEnv.APIClient()

	cID := container.Run(ctx, t, client)

	err := client.ContainerRemove(ctx, cID, types.ContainerRemoveOptions{
		Force: true,
	})
	assert.NilError(t, err)
}

func TestRemoveInvalidContainer(t *testing.T) {
	defer setupTest(t)()
	ctx := context.Background()
	client := testEnv.APIClient()

	err := client.ContainerRemove(ctx, "unknown", types.ContainerRemoveOptions{})
	assert.Check(t, is.ErrorContains(err, "No such container"))
}

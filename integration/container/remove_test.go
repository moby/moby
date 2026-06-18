package container

import (
	"os"
	"strings"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/integration/internal/container"
	"github.com/moby/moby/v2/internal/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/fs"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

// dPath converts linux absolute paths to Windows absolute paths if the daemon
// is running on Windows
func dPath(path string) string {
	if testEnv.DaemonInfo.OSType == "windows" {
		return `c:` + strings.ReplaceAll(path, "/", `\`)
	}
	return path
}

// Test case for #5244: `docker rm` fails if bind dir doesn't exist anymore
func TestRemoveContainerWithRemovedVolume(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	tempDir := fs.NewDir(t, "test-rm-container-with-removed-volume", fs.WithMode(0o755))

	cID := container.Run(ctx, t, apiClient, container.WithCmd("true"), container.WithBind(tempDir.Path(), dPath("/test")))
	var pollOps []poll.SettingOp
	if testEnv.DaemonInfo.OSType == "windows" {
		pollOps = append(pollOps, poll.WithTimeout(StopContainerWindowsPollTimeout))
	}
	poll.WaitOn(t, container.IsInState(ctx, apiClient, cID, containertypes.StateExited), pollOps...)

	err := os.RemoveAll(tempDir.Path())
	assert.NilError(t, err)

	_, err = apiClient.ContainerRemove(ctx, cID, client.ContainerRemoveOptions{
		RemoveVolumes: true,
	})
	assert.NilError(t, err)

	_, err = apiClient.ContainerInspect(ctx, cID, client.ContainerInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
	assert.Check(t, is.ErrorContains(err, "No such container"))
}

// Test case for #2099/#2125
func TestRemoveContainerWithVolume(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	cID := container.Run(ctx, t, apiClient, container.WithVolume(dPath("/srv")))

	inspect, err := apiClient.ContainerInspect(ctx, cID, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(1, len(inspect.Container.Mounts)))
	volName := inspect.Container.Mounts[0].Name

	_, err = apiClient.VolumeInspect(ctx, volName, client.VolumeInspectOptions{})
	assert.NilError(t, err)

	_, err = apiClient.ContainerRemove(ctx, cID, client.ContainerRemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})
	assert.NilError(t, err)

	_, err = apiClient.VolumeInspect(ctx, volName, client.VolumeInspectOptions{})
	assert.ErrorType(t, err, cerrdefs.IsNotFound, "Expected anonymous volume to be removed")
}

func TestRemoveContainerRunning(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	cID := container.Run(ctx, t, apiClient)

	_, err := apiClient.ContainerRemove(ctx, cID, client.ContainerRemoveOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsConflict))
	assert.Check(t, is.ErrorContains(err, "container is running"))
}

func TestRemoveContainerForceRemoveRunning(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	cID := container.Run(ctx, t, apiClient)

	_, err := apiClient.ContainerRemove(ctx, cID, client.ContainerRemoveOptions{
		Force: true,
	})
	assert.NilError(t, err)
}

func TestRemoveInvalidContainer(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	_, err := apiClient.ContainerRemove(ctx, "unknown", client.ContainerRemoveOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
	assert.Check(t, is.ErrorContains(err, "No such container"))
}

func TestRemoveDeadContainersOnDaemonRestart(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "FIXME: Windows CI does not support multiple daemons yet")

	ctx := setupTest(t)
	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)

	apiClient := d.NewClientT(t)
	container.Run(ctx, t, apiClient, container.WithCmd("top"), container.WithAutoRemove)

	list, err := apiClient.ContainerList(ctx, client.ContainerListOptions{All: true})
	assert.NilError(t, err)
	assert.Check(t, is.Len(list.Items, 1))

	d.Restart(t)

	list, err = apiClient.ContainerList(ctx, client.ContainerListOptions{All: true})
	assert.NilError(t, err)
	assert.Check(t, is.Len(list.Items, 0))
}

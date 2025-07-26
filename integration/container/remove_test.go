package container

import (
	"os"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/integration/internal/container"
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

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	prefix, slash := getPrefixAndSlashFromDaemonPlatform()

	tempDir := fs.NewDir(t, "test-rm-container-with-removed-volume", fs.WithMode(0o755))

	cID := container.Run(ctx, t, apiClient, container.WithCmd("true"), container.WithBind(tempDir.Path(), prefix+slash+"test"))
	poll.WaitOn(t, container.IsInState(ctx, apiClient, cID, containertypes.StateExited))

	err := os.RemoveAll(tempDir.Path())
	assert.NilError(t, err)

	err = apiClient.ContainerRemove(ctx, cID, containertypes.RemoveOptions{
		RemoveVolumes: true,
	})
	assert.NilError(t, err)

	_, err = apiClient.ContainerInspect(ctx, cID)
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
	assert.Check(t, is.ErrorContains(err, "No such container"))
}

// Test case for #2099/#2125
func TestRemoveContainerWithVolume(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	prefix, slash := getPrefixAndSlashFromDaemonPlatform()

	cID := container.Run(ctx, t, apiClient, container.WithVolume(prefix+slash+"srv"))

	ctrInspect, err := apiClient.ContainerInspect(ctx, cID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(1, len(ctrInspect.Mounts)))
	volName := ctrInspect.Mounts[0].Name

	_, err = apiClient.VolumeInspect(ctx, volName)
	assert.NilError(t, err)

	err = apiClient.ContainerRemove(ctx, cID, containertypes.RemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})
	assert.NilError(t, err)

	_, err = apiClient.VolumeInspect(ctx, volName)
	assert.ErrorType(t, err, cerrdefs.IsNotFound, "Expected anonymous volume to be removed")
}

func TestRemoveContainerRunning(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	cID := container.Run(ctx, t, apiClient)

	err := apiClient.ContainerRemove(ctx, cID, containertypes.RemoveOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsConflict))
	assert.Check(t, is.ErrorContains(err, "container is running"))
}

func TestRemoveContainerForceRemoveRunning(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	cID := container.Run(ctx, t, apiClient)

	err := apiClient.ContainerRemove(ctx, cID, containertypes.RemoveOptions{
		Force: true,
	})
	assert.NilError(t, err)
}

func TestRemoveInvalidContainer(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	err := apiClient.ContainerRemove(ctx, "unknown", containertypes.RemoveOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
	assert.Check(t, is.ErrorContains(err, "No such container"))
}

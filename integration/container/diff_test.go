package container

import (
	"testing"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/integration/internal/container"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestDiff(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "cannot diff a running container on Windows")
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	cID := container.Run(ctx, t, apiClient, container.WithCmd("sh", "-c", `mkdir /foo; echo xyzzy > /foo/bar`))

	expected := []containertypes.FilesystemChange{
		{Kind: containertypes.ChangeAdd, Path: "/foo"},
		{Kind: containertypes.ChangeAdd, Path: "/foo/bar"},
	}

	poll.WaitOn(t, container.IsStopped(ctx, apiClient, cID))
	items, err := apiClient.ContainerDiff(ctx, cID)
	assert.NilError(t, err)
	assert.DeepEqual(t, expected, items)
}

func TestDiffStoppedContainer(t *testing.T) {
	// There's no way in Windows to differentiate between an Add or a Modify,
	// and all files are under a "Files/" prefix.
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "FIXME")
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	cID := container.Run(ctx, t, apiClient, container.WithCmd("sh", "-c", `mkdir /foo; echo xyzzy > /foo/bar`))

	poll.WaitOn(t, container.IsInState(ctx, apiClient, cID, containertypes.StateExited), poll.WithTimeout(60*time.Second))

	expected := []containertypes.FilesystemChange{
		{Kind: containertypes.ChangeAdd, Path: "/foo"},
		{Kind: containertypes.ChangeAdd, Path: "/foo/bar"},
	}
	if testEnv.DaemonInfo.OSType == "windows" {
		expected = []containertypes.FilesystemChange{
			{Kind: containertypes.ChangeModify, Path: "Files/foo"},
			{Kind: containertypes.ChangeModify, Path: "Files/foo/bar"},
		}
	}

	items, err := apiClient.ContainerDiff(ctx, cID)
	assert.NilError(t, err)
	assert.DeepEqual(t, expected, items)
}

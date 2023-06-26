package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration/internal/container"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestDiff(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "FIXME")
	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	cID := container.Run(ctx, t, client, container.WithCmd("sh", "-c", `mkdir /foo; echo xyzzy > /foo/bar`))

	// Wait for it to exit as cannot diff a running container on Windows, and
	// it will take a few seconds to exit. Also there's no way in Windows to
	// differentiate between an Add or a Modify, and all files are under
	// a "Files/" prefix.
	expected := []containertypes.FilesystemChange{
		{Kind: containertypes.ChangeAdd, Path: "/foo"},
		{Kind: containertypes.ChangeAdd, Path: "/foo/bar"},
	}
	if testEnv.DaemonInfo.OSType == "windows" {
		poll.WaitOn(t, container.IsInState(ctx, client, cID, "exited"), poll.WithDelay(100*time.Millisecond), poll.WithTimeout(60*time.Second))
		expected = []containertypes.FilesystemChange{
			{Kind: containertypes.ChangeModify, Path: "Files/foo"},
			{Kind: containertypes.ChangeModify, Path: "Files/foo/bar"},
		}
	}

	items, err := client.ContainerDiff(ctx, cID)
	assert.NilError(t, err)
	assert.DeepEqual(t, expected, items)
}

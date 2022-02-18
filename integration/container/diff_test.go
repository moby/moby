package container // import "github.com/moby/moby/integration/container"

import (
	"context"
	"testing"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/integration/internal/container"
	"github.com/moby/moby/pkg/archive"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestDiff(t *testing.T) {
	skip.If(t, testEnv.OSType == "windows", "FIXME")
	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	cID := container.Run(ctx, t, client, container.WithCmd("sh", "-c", `mkdir /foo; echo xyzzy > /foo/bar`))

	// Wait for it to exit as cannot diff a running container on Windows, and
	// it will take a few seconds to exit. Also there's no way in Windows to
	// differentiate between an Add or a Modify, and all files are under
	// a "Files/" prefix.
	expected := []containertypes.ContainerChangeResponseItem{
		{Kind: archive.ChangeAdd, Path: "/foo"},
		{Kind: archive.ChangeAdd, Path: "/foo/bar"},
	}
	if testEnv.OSType == "windows" {
		poll.WaitOn(t, container.IsInState(ctx, client, cID, "exited"), poll.WithDelay(100*time.Millisecond), poll.WithTimeout(60*time.Second))
		expected = []containertypes.ContainerChangeResponseItem{
			{Kind: archive.ChangeModify, Path: "Files/foo"},
			{Kind: archive.ChangeModify, Path: "Files/foo/bar"},
		}
	}

	items, err := client.ContainerDiff(ctx, cID)
	assert.NilError(t, err)
	assert.DeepEqual(t, expected, items)
}

package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/request"
	"github.com/docker/docker/pkg/archive"
	"github.com/gotestyourself/gotestyourself/poll"
	"github.com/gotestyourself/gotestyourself/skip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ensure that an added file shows up in docker diff
func TestDiffFilenameShownInOutput(t *testing.T) {
	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	cID := container.Run(t, ctx, client, container.WithCmd("sh", "-c", `mkdir /foo; echo xyzzy > /foo/bar`))

	// Wait for it to exit as cannot diff a running container on Windows, and
	// it will take a few seconds to exit. Also there's no way in Windows to
	// differentiate between an Add or a Modify, and all files are under
	// a "Files/" prefix.
	lookingFor := containertypes.ContainerChangeResponseItem{Kind: archive.ChangeAdd, Path: "/foo/bar"}
	if testEnv.OSType == "windows" {
		poll.WaitOn(t, containerIsInState(ctx, client, cID, "exited"), poll.WithDelay(100*time.Millisecond), poll.WithTimeout(60*time.Second))
		lookingFor = containertypes.ContainerChangeResponseItem{Kind: archive.ChangeModify, Path: "Files/foo/bar"}
	}

	items, err := client.ContainerDiff(ctx, cID)
	require.NoError(t, err)
	assert.Contains(t, items, lookingFor)
}

// test to ensure GH #3840 doesn't occur any more
func TestDiffEnsureInitLayerFilesAreIgnored(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	// this is a list of files which shouldn't show up in `docker diff`
	initLayerFiles := []string{"/etc/resolv.conf", "/etc/hostname", "/etc/hosts", "/.dockerenv"}
	containerCount := 5

	// we might not run into this problem from the first run, so start a few containers
	for i := 0; i < containerCount; i++ {
		cID := container.Run(t, ctx, client, container.WithCmd("sh", "-c", `echo foo > /root/bar`))

		items, err := client.ContainerDiff(ctx, cID)
		require.NoError(t, err)
		for _, item := range items {
			assert.NotContains(t, initLayerFiles, item.Path)
		}
	}
}

func TestDiffEnsureDefaultDevs(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	cID := container.Run(t, ctx, client, container.WithCmd("sleep", "0"))

	items, err := client.ContainerDiff(ctx, cID)
	require.NoError(t, err)

	expected := []containertypes.ContainerChangeResponseItem{
		{Kind: archive.ChangeModify, Path: "/dev"},
		{Kind: archive.ChangeAdd, Path: "/dev/full"},    // busybox
		{Kind: archive.ChangeModify, Path: "/dev/ptmx"}, // libcontainer
		{Kind: archive.ChangeAdd, Path: "/dev/mqueue"},
		{Kind: archive.ChangeAdd, Path: "/dev/kmsg"},
		{Kind: archive.ChangeAdd, Path: "/dev/fd"},
		{Kind: archive.ChangeAdd, Path: "/dev/ptmx"},
		{Kind: archive.ChangeAdd, Path: "/dev/null"},
		{Kind: archive.ChangeAdd, Path: "/dev/random"},
		{Kind: archive.ChangeAdd, Path: "/dev/stdout"},
		{Kind: archive.ChangeAdd, Path: "/dev/stderr"},
		{Kind: archive.ChangeAdd, Path: "/dev/tty1"},
		{Kind: archive.ChangeAdd, Path: "/dev/stdin"},
		{Kind: archive.ChangeAdd, Path: "/dev/tty"},
		{Kind: archive.ChangeAdd, Path: "/dev/urandom"},
	}

	for _, item := range items {
		assert.Contains(t, expected, item)
	}
}

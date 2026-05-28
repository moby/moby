package container

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	mounttypes "github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/integration/internal/build"
	"github.com/moby/moby/v2/integration/internal/container"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/fakecontext"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

// TestCopyWithAbsoluteSymlinkedMountTarget was introduced as a regression test
// for https://github.com/moby/moby/issues/52653.
//
// The security fix in GHSA-vp62-88p7-qqf5 switched openContainerFS to use
// os.Root for the mount-destination operations.
// os.Root refuses to follow absolute symlinks, but distro images commonly ship
// /var/run as an absolute symlink to /run.
// As a result, any container with a bind mount whose target traversed such a
// symlink (e.g. -v /host/sock:/var/run/docker.sock) made `docker cp` fail.
func TestCopyWithAbsoluteSymlinkedMountTarget(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	// Build an image with an absolute in-container symlink along the mount
	// target path.
	// Stock distro images expose this shape via /var/run -> /run, but we set
	// up our own /sockets -> /root pair so the test does not depend on any
	// particular base image's layout.
	buildCtx := fakecontext.New(t, "",
		fakecontext.WithDockerfile(`FROM busybox
RUN touch /root/nil && ln -s /root /sockets
`),
	)
	defer buildCtx.Close()
	imgID := build.Do(ctx, t, apiClient, buildCtx, client.ImageBuildOptions{})

	// Use testutil.TempDir so the rootless daemon can access the bind-mount
	// source: t.TempDir() creates a 0700 parent that the fake-root user
	// cannot stat.
	srcDir := testutil.TempDir(t)
	assert.NilError(t, os.WriteFile(filepath.Join(srcDir, "sock"), nil, 0o644))

	cid := container.Create(ctx, t, apiClient,
		container.WithImage(imgID),
		container.WithMount(mounttypes.Mount{
			Type:   mounttypes.TypeBind,
			Source: filepath.Join(srcDir, "sock"),
			Target: "/sockets/docker.sock",
		}),
	)

	_, err := apiClient.CopyToContainer(ctx, cid, client.CopyToContainerOptions{
		DestinationPath: "/sockets/",
		Content:         bytes.NewReader(nil),
	})
	assert.NilError(t, err)
}

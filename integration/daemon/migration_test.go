package daemon // import "github.com/docker/docker/integration/daemon"

import (
	"os"
	"runtime"
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/daemon"
	"github.com/docker/docker/testutil/fixtures/load"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

func TestMigrateOverlaySnapshotter(t *testing.T) {
	testMigrateSnapshotter(t, "overlay2", "overlayfs")
}

func TestMigrateNativeSnapshotter(t *testing.T) {
	t.Skip("vfs migration not implemented yet")
	testMigrateSnapshotter(t, "vfs", "native")
}

func testMigrateSnapshotter(t *testing.T, graphdriver, snapshotter string) {
	skip.If(t, runtime.GOOS != "linux")
	skip.If(t, os.Getenv("TEST_INTEGRATION_USE_SNAPSHOTTER") != "")

	ctx := testutil.StartSpan(baseContext, t)

	d := daemon.New(t)
	defer d.Stop(t)

	d.Start(t, "--iptables=false", "--ip6tables=false", "-s", graphdriver)
	info := d.Info(t)
	id := info.ID
	assert.Check(t, id != "")
	assert.Equal(t, info.Containers, 0)
	assert.Equal(t, info.Images, 0)
	assert.Equal(t, info.Driver, graphdriver)

	load.FrozenImagesLinux(ctx, d.NewClientT(t), "busybox:latest")

	info = d.Info(t)
	allImages := info.Images
	assert.Check(t, allImages > 0)

	apiClient := d.NewClientT(t)

	containerID := container.Run(ctx, t, apiClient, func(c *container.TestContainerConfig) {
		c.Name = "Migration-1-" + snapshotter
		c.Config.Image = "busybox:latest"
		c.Config.Cmd = []string{"top"}
	})

	d.Stop(t)

	// Start with migration feature but with a container which will prevent migration
	d.Start(t, "--iptables=false", "--ip6tables=false", "-s", graphdriver, "--feature", "containerd-migration")
	info = d.Info(t)
	assert.Equal(t, info.ID, id)
	assert.Equal(t, info.Driver, graphdriver)
	assert.Equal(t, info.Containers, 1)
	assert.Equal(t, info.Images, allImages)
	container.Remove(ctx, t, apiClient, containerID, containertypes.RemoveOptions{
		Force: true,
	})

	d.Stop(t)

	d.Start(t, "--iptables=false", "--ip6tables=false", "-s", graphdriver, "--feature", "containerd-migration")
	info = d.Info(t)
	assert.Equal(t, info.ID, id)
	assert.Equal(t, info.Containers, 0)
	assert.Equal(t, info.Driver, snapshotter, "expected migrate to switch from %s to %s", graphdriver, snapshotter)
	assert.Equal(t, info.Images, allImages)

	result := container.RunAttach(ctx, t, apiClient, func(c *container.TestContainerConfig) {
		c.Name = "Migration-2-" + snapshotter
		c.Config.Image = "busybox:latest"
		c.Config.Cmd = []string{"echo", "hello"}
	})
	assert.Equal(t, result.ExitCode, 0)
	container.Remove(ctx, t, apiClient, result.ContainerID, containertypes.RemoveOptions{})
}

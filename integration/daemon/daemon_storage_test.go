package daemon

import (
	"strings"
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/daemon"
	"github.com/moby/sys/mountinfo"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

// TestOverlayForceIndexOnStorageOpt tests that a newly created container's
// filesystem layers have been mounted without `index=off` when the storage
// option `overlay2.force_index_on` is `true`.
func TestOverlayForceIndexOnStorageOpt(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "The overlay2 storage driver is not supported on windows")
	t.Setenv("DOCKER_DRIVER", "")
	t.Setenv("DOCKER_GRAPHDRIVER", "")
	t.Setenv("TEST_INTEGRATION_USE_GRAPHDRIVER", "")

	ctx := testutil.StartSpan(baseContext, t)
	d := daemon.New(t, daemon.WithStorageDriver("overlay2"))
	t.Cleanup(func() {
		d.Stop(t)
	})

	const testImage = "busybox:latest"
	d.StartWithBusybox(ctx, t, "--storage-driver", "overlay2", "--storage-opt", "overlay2.force_index_on=true")
	c := d.NewClientT(t)

	containerResp, err := c.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config: &container.Config{
			Image: testImage,
			Cmd:   []string{"sh", "-c", "while true; do echo 'Celeste is a cute cat'; sleep 5; done"},
		},
	})
	assert.NilError(t, err, "Failed to create container")
	cid := containerResp.ID
	_, err = c.ContainerStart(ctx, cid, client.ContainerStartOptions{})
	assert.NilError(t, err, "Failed to start container")

	inspected, err := c.ContainerInspect(ctx, cid, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	mergedDir, ok := inspected.Container.GraphDriver.Data["MergedDir"]
	assert.Check(t, ok, "Merged dir should be present with the overlay2 storage driver")
	mnts, err := mountinfo.GetMounts(func(i *mountinfo.Info) (skip bool, stop bool) {
		skip = i.FSType != "overlay" || i.Mountpoint != mergedDir
		return
	})	
	assert.NilError(t, err, "Failed to get overlay mounts")
	assert.Check(t, len(mnts) > 0, "Overlay mount point for container not found")
	if len(mnts) > 0 {
		assert.Check(t, !strings.Contains(mnts[0].VFSOptions, "index=off"), "Should not be mounted with `index=off`")
	}
	_, err = c.ContainerRemove(ctx, cid, client.ContainerRemoveOptions{
		Force: true,
	})
	assert.NilError(t, err, "Failed to remove container")
	d.Stop(t)
}

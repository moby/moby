package daemon

import (
	"testing"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/testutil"
	"github.com/moby/moby/v2/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestDefaultStorageDriver(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "Windows does not support running sub-daemons")
	t.Setenv("DOCKER_DRIVER", "")
	t.Setenv("DOCKER_GRAPHDRIVER", "")
	t.Setenv("TEST_INTEGRATION_USE_GRAPHDRIVER", "")
	_ = testutil.StartSpan(baseContext, t)

	d := daemon.New(t)
	defer d.Stop(t)

	d.Start(t, "--iptables=false", "--ip6tables=false")

	info := d.Info(t)
	assert.Check(t, is.Equal(info.DriverStatus[0][1], "io.containerd.snapshotter.v1"))
}

// TestGraphDriverPersistence tests that when a daemon starts with graphdrivers,
// pulls images and creates containers, then is restarted without explicit
// graphdriver configuration, it continues to use graphdrivers instead of
// migrating to containerd snapshotters automatically.
func TestGraphDriverPersistence(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "Windows does not support running sub-daemons")
	t.Setenv("DOCKER_DRIVER", "")
	t.Setenv("DOCKER_GRAPHDRIVER", "")
	t.Setenv("TEST_INTEGRATION_USE_GRAPHDRIVER", "")
	ctx := testutil.StartSpan(baseContext, t)

	// Phase 1: Start daemon with explicit graphdriver (overlay2)
	d := daemon.New(t)
	t.Cleanup(func() {
		d.Stop(t)
	})

	const testImage = "busybox:latest"
	d.StartWithBusybox(ctx, t, "--iptables=false", "--ip6tables=false", "--storage-driver=overlay2")
	c := d.NewClientT(t)

	// Verify we're using graphdriver
	info := d.Info(t)
	assert.Check(t, info.DriverStatus[0][1] != "io.containerd.snapshotter.v1")
	prevDriver := info.Driver

	containerResp, err := c.ContainerCreate(ctx, &containertypes.Config{
		Image: testImage,
		Cmd:   []string{"echo", "test"},
	}, nil, nil, nil, "test-container")
	assert.NilError(t, err, "Failed to create container")

	containerID := containerResp.ID

	d.Stop(t)

	// Phase 2: Start daemon again WITHOUT explicit graphdriver configuration
	d.Start(t, "--iptables=false", "--ip6tables=false")

	// Verify daemon still uses graphdriver (not containerd snapshotter)
	// Verify we're using graphdriver
	info = d.Info(t)
	assert.Check(t, info.DriverStatus[0][1] != "io.containerd.snapshotter.v1")
	assert.Check(t, is.Equal(info.Driver, prevDriver))

	// Verify our image is still there
	_, err = c.ImageInspect(ctx, testImage)
	assert.NilError(t, err, "Test image should still be available after daemon restart")

	// Verify our container is still there
	_, err = c.ContainerInspect(ctx, containerID)
	assert.NilError(t, err, "Test container should still exist after daemon restart")
}

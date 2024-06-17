package system

import (
	"os"
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/daemon"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

// hasSystemd checks whether the host was booted with systemd as its init
// system. Stolen from
// https://github.com/coreos/go-systemd/blob/176f85496f4e/util/util.go#L68
func hasSystemd() bool {
	fi, err := os.Lstat("/run/systemd/system")
	if err != nil {
		return false
	}
	return fi.IsDir()
}

// TestCgroupDriverSystemdMemoryLimit checks that container
// memory limit can be set when using systemd cgroupdriver.
// https://github.com/moby/moby/issues/35123
func TestCgroupDriverSystemdMemoryLimit(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, !hasSystemd())
	t.Parallel()

	ctx := testutil.StartSpan(baseContext, t)

	d := daemon.New(t)
	c := d.NewClientT(t)

	d.StartWithBusybox(ctx, t, "--exec-opt", "native.cgroupdriver=systemd", "--iptables=false", "--ip6tables=false")
	defer d.Stop(t)

	const mem = int64(64 * 1024 * 1024) // 64 MB

	ctrID := container.Create(ctx, t, c, func(ctr *container.TestContainerConfig) {
		ctr.HostConfig.Resources.Memory = mem
	})
	defer c.ContainerRemove(ctx, ctrID, containertypes.RemoveOptions{Force: true})

	err := c.ContainerStart(ctx, ctrID, containertypes.StartOptions{})
	assert.NilError(t, err)

	s, err := c.ContainerInspect(ctx, ctrID)
	assert.NilError(t, err)
	assert.Equal(t, s.HostConfig.Memory, mem)
}

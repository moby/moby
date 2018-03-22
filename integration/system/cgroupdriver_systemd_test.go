package system

import (
	"context"
	"os"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration-cli/daemon"

	"github.com/gotestyourself/gotestyourself/assert"
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
//  https://github.com/moby/moby/issues/35123
func TestCgroupDriverSystemdMemoryLimit(t *testing.T) {
	t.Parallel()

	if !hasSystemd() {
		t.Skip("systemd not available")
	}

	d := daemon.New(t, "docker", "dockerd", daemon.Config{})
	client, err := d.NewClient()
	assert.NilError(t, err)
	d.StartWithBusybox(t, "--exec-opt", "native.cgroupdriver=systemd", "--iptables=false")
	defer d.Stop(t)

	const mem = 64 * 1024 * 1024 // 64 MB
	cfg := container.Config{
		Image: "busybox",
		Cmd:   []string{"top"},
	}
	hostcfg := container.HostConfig{
		Resources: container.Resources{
			Memory: mem,
		},
	}

	ctx := context.Background()
	ctr, err := client.ContainerCreate(ctx, &cfg, &hostcfg, nil, "")
	assert.NilError(t, err)
	defer client.ContainerRemove(ctx, ctr.ID, types.ContainerRemoveOptions{Force: true})

	err = client.ContainerStart(ctx, ctr.ID, types.ContainerStartOptions{})
	assert.NilError(t, err)

	s, err := client.ContainerInspect(ctx, ctr.ID)
	assert.NilError(t, err)
	assert.Equal(t, s.HostConfig.Memory, mem)
}

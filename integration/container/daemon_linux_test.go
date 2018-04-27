package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/internal/test/daemon"
	"github.com/gotestyourself/gotestyourself/assert"
	"github.com/gotestyourself/gotestyourself/skip"
	"golang.org/x/sys/unix"
)

// This is a regression test for #36145
// It ensures that a container can be started when the daemon was improperly
// shutdown when the daemon is brought back up.
//
// The regression is due to improper error handling preventing a container from
// being restored and as such have the resources cleaned up.
//
// To test this, we need to kill dockerd, then kill both the containerd-shim and
// the container process, then start dockerd back up and attempt to start the
// container again.
func TestContainerStartOnDaemonRestart(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot start daemon on remote test run")
	t.Parallel()

	d := daemon.New(t)
	d.StartWithBusybox(t, "--iptables=false")
	defer d.Stop(t)

	client, err := d.NewClient()
	assert.Check(t, err, "error creating client")

	ctx := context.Background()

	cID := container.Create(t, ctx, client)
	defer client.ContainerRemove(ctx, cID, types.ContainerRemoveOptions{Force: true})

	err = client.ContainerStart(ctx, cID, types.ContainerStartOptions{})
	assert.Check(t, err, "error starting test container")

	inspect, err := client.ContainerInspect(ctx, cID)
	assert.Check(t, err, "error getting inspect data")

	ppid := getContainerdShimPid(t, inspect)

	err = d.Kill()
	assert.Check(t, err, "failed to kill test daemon")

	err = unix.Kill(inspect.State.Pid, unix.SIGKILL)
	assert.Check(t, err, "failed to kill container process")

	err = unix.Kill(ppid, unix.SIGKILL)
	assert.Check(t, err, "failed to kill containerd-shim")

	d.Start(t, "--iptables=false")

	err = client.ContainerStart(ctx, cID, types.ContainerStartOptions{})
	assert.Check(t, err, "failed to start test container")
}

func getContainerdShimPid(t *testing.T, c types.ContainerJSON) int {
	statB, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/stat", c.State.Pid))
	assert.Check(t, err, "error looking up containerd-shim pid")

	// ppid is the 4th entry in `/proc/pid/stat`
	ppid, err := strconv.Atoi(strings.Fields(string(statB))[3])
	assert.Check(t, err, "error converting ppid field to int")

	assert.Check(t, ppid != 1, "got unexpected ppid")
	return ppid
}

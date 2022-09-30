package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	containerapi "github.com/docker/docker/api/types/container"
	realcontainer "github.com/docker/docker/container"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/testutil/daemon"
	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
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
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, testEnv.IsRootless)
	t.Parallel()

	d := daemon.New(t)
	d.StartWithBusybox(t, "--iptables=false")
	defer d.Stop(t)

	c := d.NewClientT(t)

	ctx := context.Background()

	cID := container.Create(ctx, t, c)
	defer c.ContainerRemove(ctx, cID, types.ContainerRemoveOptions{Force: true})

	err := c.ContainerStart(ctx, cID, types.ContainerStartOptions{})
	assert.Check(t, err, "error starting test container")

	inspect, err := c.ContainerInspect(ctx, cID)
	assert.Check(t, err, "error getting inspect data")

	ppid := getContainerdShimPid(t, inspect)

	err = d.Kill()
	assert.Check(t, err, "failed to kill test daemon")

	err = unix.Kill(inspect.State.Pid, unix.SIGKILL)
	assert.Check(t, err, "failed to kill container process")

	err = unix.Kill(ppid, unix.SIGKILL)
	assert.Check(t, err, "failed to kill containerd-shim")

	d.Start(t, "--iptables=false")

	err = c.ContainerStart(ctx, cID, types.ContainerStartOptions{})
	assert.Check(t, err, "failed to start test container")
}

func getContainerdShimPid(t *testing.T, c types.ContainerJSON) int {
	statB, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", c.State.Pid))
	assert.Check(t, err, "error looking up containerd-shim pid")

	// ppid is the 4th entry in `/proc/pid/stat`
	ppid, err := strconv.Atoi(strings.Fields(string(statB))[3])
	assert.Check(t, err, "error converting ppid field to int")

	assert.Check(t, ppid != 1, "got unexpected ppid")
	return ppid
}

// TestDaemonRestartIpcMode makes sure a container keeps its ipc mode
// (derived from daemon default) even after the daemon is restarted
// with a different default ipc mode.
func TestDaemonRestartIpcMode(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot start daemon on remote test run")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	t.Parallel()

	d := daemon.New(t)
	d.StartWithBusybox(t, "--iptables=false", "--default-ipc-mode=private")
	defer d.Stop(t)

	c := d.NewClientT(t)
	ctx := context.Background()

	// check the container is created with private ipc mode as per daemon default
	cID := container.Run(ctx, t, c,
		container.WithCmd("top"),
		container.WithRestartPolicy("always"),
	)
	defer c.ContainerRemove(ctx, cID, types.ContainerRemoveOptions{Force: true})

	inspect, err := c.ContainerInspect(ctx, cID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(string(inspect.HostConfig.IpcMode), "private"))

	// restart the daemon with shareable default ipc mode
	d.Restart(t, "--iptables=false", "--default-ipc-mode=shareable")

	// check the container is still having private ipc mode
	inspect, err = c.ContainerInspect(ctx, cID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(string(inspect.HostConfig.IpcMode), "private"))

	// check a new container is created with shareable ipc mode as per new daemon default
	cID = container.Run(ctx, t, c)
	defer c.ContainerRemove(ctx, cID, types.ContainerRemoveOptions{Force: true})

	inspect, err = c.ContainerInspect(ctx, cID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(string(inspect.HostConfig.IpcMode), "shareable"))
}

// TestDaemonHostGatewayIP verifies that when a magic string "host-gateway" is passed
// to ExtraHosts (--add-host) instead of an IP address, its value is set to
// 1. Daemon config flag value specified by host-gateway-ip or
// 2. IP of the default bridge network
// and is added to the /etc/hosts file
func TestDaemonHostGatewayIP(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, testEnv.IsRootless, "rootless mode has different view of network")
	t.Parallel()

	// Verify the IP in /etc/hosts is same as host-gateway-ip
	d := daemon.New(t)
	// Verify the IP in /etc/hosts is same as the default bridge's IP
	d.StartWithBusybox(t)
	c := d.NewClientT(t)
	ctx := context.Background()
	cID := container.Run(ctx, t, c,
		container.WithExtraHost("host.docker.internal:host-gateway"),
	)
	res, err := container.Exec(ctx, c, cID, []string{"cat", "/etc/hosts"})
	assert.NilError(t, err)
	assert.Assert(t, is.Len(res.Stderr(), 0))
	assert.Equal(t, 0, res.ExitCode)
	inspect, err := c.NetworkInspect(ctx, "bridge", types.NetworkInspectOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Contains(res.Stdout(), inspect.IPAM.Config[0].Gateway))
	c.ContainerRemove(ctx, cID, types.ContainerRemoveOptions{Force: true})
	d.Stop(t)

	// Verify the IP in /etc/hosts is same as host-gateway-ip
	d.StartWithBusybox(t, "--host-gateway-ip=6.7.8.9")
	cID = container.Run(ctx, t, c,
		container.WithExtraHost("host.docker.internal:host-gateway"),
	)
	res, err = container.Exec(ctx, c, cID, []string{"cat", "/etc/hosts"})
	assert.NilError(t, err)
	assert.Assert(t, is.Len(res.Stderr(), 0))
	assert.Equal(t, 0, res.ExitCode)
	assert.Check(t, is.Contains(res.Stdout(), "6.7.8.9"))
	c.ContainerRemove(ctx, cID, types.ContainerRemoveOptions{Force: true})
	d.Stop(t)
}

// TestRestartDaemonWithRestartingContainer simulates a case where a container is in "restarting" state when
// dockerd is killed (due to machine reset or something else).
//
// Related to moby/moby#41817
//
// In this test we'll change the container state to "restarting".
// This means that the container will not be 'alive' when we attempt to restore in on daemon startup.
//
// We could do the same with `docker run -d --resetart=always busybox:latest exit 1`, and then
// `kill -9` dockerd while the container is in "restarting" state. This is difficult to reproduce reliably
// in an automated test, so we manipulate on disk state instead.
func TestRestartDaemonWithRestartingContainer(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot start daemon on remote test run")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

	t.Parallel()

	d := daemon.New(t)
	defer d.Cleanup(t)

	d.StartWithBusybox(t, "--iptables=false")
	defer d.Stop(t)

	ctx := context.Background()
	client := d.NewClientT(t)

	// Just create the container, no need to start it to be started.
	// We really want to make sure there is no process running when docker starts back up.
	// We will manipulate the on disk state later
	id := container.Create(ctx, t, client, container.WithRestartPolicy("always"), container.WithCmd("/bin/sh", "-c", "exit 1"))

	d.Stop(t)

	configPath := filepath.Join(d.Root, "containers", id, "config.v2.json")
	configBytes, err := os.ReadFile(configPath)
	assert.NilError(t, err)

	var c realcontainer.Container

	assert.NilError(t, json.Unmarshal(configBytes, &c))

	c.State = realcontainer.NewState()
	c.SetRestarting(&realcontainer.ExitStatus{ExitCode: 1})
	c.HasBeenStartedBefore = true

	configBytes, err = json.Marshal(&c)
	assert.NilError(t, err)
	assert.NilError(t, os.WriteFile(configPath, configBytes, 0600))

	d.Start(t)

	ctxTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	chOk, chErr := client.ContainerWait(ctxTimeout, id, containerapi.WaitConditionNextExit)
	select {
	case <-chOk:
	case err := <-chErr:
		assert.NilError(t, err)
	}
}

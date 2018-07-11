package network // import "github.com/docker/docker/integration/network"

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/internal/test/daemon"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	"gotest.tools/skip"
)

func TestRunContainerWithBridgeNone(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot start daemon on remote test run")
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, IsUserNamespace())

	d := daemon.New(t)
	d.StartWithBusybox(t, "-b", "none")
	defer d.Stop(t)

	client, err := d.NewClient()
	assert.Check(t, err, "error creating client")

	ctx := context.Background()
	id1 := container.Run(t, ctx, client)
	defer client.ContainerRemove(ctx, id1, types.ContainerRemoveOptions{Force: true})

	result, err := container.Exec(ctx, client, id1, []string{"ip", "l"})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(false, strings.Contains(result.Combined(), "eth0")), "There shouldn't be eth0 in container in default(bridge) mode when bridge network is disabled")

	id2 := container.Run(t, ctx, client, container.WithNetworkMode("bridge"))
	defer client.ContainerRemove(ctx, id2, types.ContainerRemoveOptions{Force: true})

	result, err = container.Exec(ctx, client, id2, []string{"ip", "l"})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(false, strings.Contains(result.Combined(), "eth0")), "There shouldn't be eth0 in container in bridge mode when bridge network is disabled")

	nsCommand := "ls -l /proc/self/ns/net | awk -F '->' '{print $2}'"
	cmd := exec.Command("sh", "-c", nsCommand)
	stdout := bytes.NewBuffer(nil)
	cmd.Stdout = stdout
	err = cmd.Run()
	assert.NilError(t, err, "Failed to get current process network namespace: %+v", err)

	id3 := container.Run(t, ctx, client, container.WithNetworkMode("host"))
	defer client.ContainerRemove(ctx, id3, types.ContainerRemoveOptions{Force: true})

	result, err = container.Exec(ctx, client, id3, []string{"sh", "-c", nsCommand})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(stdout.String(), result.Combined()), "The network namspace of container should be the same with host when --net=host and bridge network is disabled")
}

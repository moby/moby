package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"os/exec"
	"regexp"
	"sort"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/checkpoint"
	mounttypes "github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

//nolint:unused // false positive: linter detects this as "unused"
func containerExec(t *testing.T, client client.APIClient, cID string, cmd []string) {
	t.Logf("Exec: %s", cmd)
	ctx := context.Background()
	r, err := container.Exec(ctx, client, cID, cmd)
	assert.NilError(t, err)
	t.Log(r.Combined())
	assert.Equal(t, r.ExitCode, 0)
}

func TestCheckpoint(t *testing.T) {
	t.Skip("TestCheckpoint is broken; see https://github.com/moby/moby/issues/38963")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, !testEnv.DaemonInfo.ExperimentalBuild)

	defer setupTest(t)()

	stdoutStderr, err := exec.Command("criu", "check").CombinedOutput()
	t.Logf("%s", stdoutStderr)
	assert.NilError(t, err)

	ctx := context.Background()
	apiClient := request.NewAPIClient(t)

	t.Log("Start a container")
	cID := container.Run(ctx, t, apiClient, container.WithMount(mounttypes.Mount{
		Type:   mounttypes.TypeTmpfs,
		Target: "/tmp",
	}))
	poll.WaitOn(t,
		container.IsInState(ctx, apiClient, cID, "running"),
		poll.WithDelay(100*time.Millisecond),
	)

	// FIXME: ipv6 iptables modules are not uploaded in the test environment
	stdoutStderr, err = exec.Command("bash", "-c", "set -x; "+
		"mount --bind $(type -P true) $(type -P ip6tables-restore) && "+
		"mount --bind $(type -P true) $(type -P ip6tables-save)",
	).CombinedOutput()
	t.Logf("%s", stdoutStderr)
	assert.NilError(t, err)

	defer func() {
		stdoutStderr, err = exec.Command("bash", "-c", "set -x; "+
			"umount -c -i -l $(type -P ip6tables-restore); "+
			"umount -c -i -l $(type -P ip6tables-save)",
		).CombinedOutput()
		t.Logf("%s", stdoutStderr)
		assert.NilError(t, err)
	}()

	t.Log("Do a checkpoint and leave the container running")
	err = apiClient.CheckpointCreate(ctx, cID, checkpoint.CreateOptions{
		Exit:         false,
		CheckpointID: "test",
	})
	if err != nil {
		// An error can contain a path to a dump file
		t.Log(err)
		re := regexp.MustCompile("path= (.*): ")
		m := re.FindStringSubmatch(err.Error())
		if len(m) >= 2 {
			dumpLog := m[1]
			t.Logf("%s", dumpLog)
			cmd := exec.Command("cat", dumpLog)
			stdoutStderr, err = cmd.CombinedOutput()
			t.Logf("%s", stdoutStderr)
		}
	}
	assert.NilError(t, err)

	inspect, err := apiClient.ContainerInspect(ctx, cID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(true, inspect.State.Running))

	checkpoints, err := apiClient.CheckpointList(ctx, cID, checkpoint.ListOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(checkpoints), 1)
	assert.Equal(t, checkpoints[0].Name, "test")

	// Create a test file on a tmpfs mount.
	containerExec(t, apiClient, cID, []string{"touch", "/tmp/test-file"})

	// Do a second checkpoint
	t.Log("Do a checkpoint and stop the container")
	err = apiClient.CheckpointCreate(ctx, cID, checkpoint.CreateOptions{
		Exit:         true,
		CheckpointID: "test2",
	})
	assert.NilError(t, err)

	poll.WaitOn(t,
		container.IsInState(ctx, apiClient, cID, "exited"),
		poll.WithDelay(100*time.Millisecond),
	)

	inspect, err = apiClient.ContainerInspect(ctx, cID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(false, inspect.State.Running))

	// Check that both checkpoints are listed.
	checkpoints, err = apiClient.CheckpointList(ctx, cID, checkpoint.ListOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(checkpoints), 2)
	cptNames := make([]string, 2)
	for i, c := range checkpoints {
		cptNames[i] = c.Name
	}
	sort.Strings(cptNames)
	assert.Equal(t, cptNames[0], "test")
	assert.Equal(t, cptNames[1], "test2")

	// Restore the container from a second checkpoint.
	t.Log("Restore the container")
	err = apiClient.ContainerStart(ctx, cID, types.ContainerStartOptions{
		CheckpointID: "test2",
	})
	assert.NilError(t, err)

	inspect, err = apiClient.ContainerInspect(ctx, cID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(true, inspect.State.Running))

	// Check that the test file has been restored.
	containerExec(t, apiClient, cID, []string{"test", "-f", "/tmp/test-file"})

	for _, id := range []string{"test", "test2"} {
		err = apiClient.CheckpointDelete(ctx, cID, checkpoint.DeleteOptions{
			CheckpointID: id,
		})
		assert.NilError(t, err)
	}
}

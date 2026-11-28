package container

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/integration/internal/container"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/daemon"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

func TestStartAfterTaskDeleteTimeout(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, testEnv.IsRootless(), "test requires a rootful daemon with a custom runc runtime")

	runc, err := exec.LookPath("runc")
	assert.NilError(t, err)
	runc, err = filepath.Abs(runc)
	assert.NilError(t, err)

	dir := t.TempDir()
	arm := filepath.Join(dir, "arm")
	wrapper := filepath.Join(dir, "runc")
	assert.NilError(t, os.WriteFile(wrapper, []byte(`#!/bin/sh
for id do :; done
for arg do
	if [ "$arg" = delete ] && [ "$(cat "$MOBY_TEST_DELETE_ARM" 2>/dev/null)" = "$id" ] && rm "$MOBY_TEST_DELETE_ARM"; then
		# Exceed the daemon's first task-delete deadline once.
		sleep 36
		break
	fi
done
exec "$MOBY_TEST_REAL_RUNC" "$@"
`), 0o700))

	ctx, cancel := context.WithTimeout(testutil.StartSpan(baseContext, t), 90*time.Second)
	defer cancel()
	d := daemon.New(t, daemon.WithContainerdSocket(""), daemon.WithEnvVars(
		"MOBY_TEST_DELETE_ARM="+arm,
		"MOBY_TEST_REAL_RUNC="+runc,
	))
	d.StartWithBusybox(ctx, t, "--add-runtime=delayed="+wrapper)
	defer d.Stop(t)

	apiClient := d.NewClientT(t)
	id := container.Create(ctx, t, apiClient, container.WithRuntime("delayed"),
		container.WithNetworkMode("none"), container.WithCmd("sh", "-c", "exit 23"))

	wait := apiClient.ContainerWait(ctx, id, client.ContainerWaitOptions{Condition: containertypes.WaitConditionNextExit})
	assert.NilError(t, os.WriteFile(arm, []byte(id), 0o600))
	_, err = apiClient.ContainerStart(ctx, id, client.ContainerStartOptions{})
	assert.NilError(t, err)
	assertContainerExitCode(t, wait, 23, 60*time.Second)
	timedOut, _ := d.ScanLogsT(ctx, t, daemon.ScanLogsMatchAll(
		"failed to delete container from containerd", "deadline exceeded", id,
	))
	assert.Assert(t, timedOut, "expected the first task delete to time out")

	inspectCtx, cancelInspect := context.WithTimeout(ctx, 3*time.Second)
	inspected, err := apiClient.ContainerInspect(inspectCtx, id, client.ContainerInspectOptions{})
	cancelInspect()
	assert.NilError(t, err)
	assert.Equal(t, inspected.Container.State.ExitCode, 23)

	wait = apiClient.ContainerWait(ctx, id, client.ContainerWaitOptions{Condition: containertypes.WaitConditionNextExit})
	_, err = apiClient.ContainerStart(ctx, id, client.ContainerStartOptions{})
	assert.NilError(t, err)
	assertContainerExitCode(t, wait, 23, 30*time.Second)
}

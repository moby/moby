package container // import "github.com/docker/docker/integration/container"

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestCreateWithCDIDevices(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux", "CDI devices are only supported on Linux")
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run cdi tests with a remote daemon")

	cwd, err := os.Getwd()
	assert.NilError(t, err)
	d := daemon.New(t, daemon.WithExperimental())
	d.StartWithBusybox(t, "--cdi-spec-dir="+filepath.Join(cwd, "testdata", "cdi"))
	defer d.Stop(t)

	client := d.NewClientT(t)

	ctx := context.Background()
	id := container.Run(ctx, t, client,
		container.WithCmd("/bin/sh", "-c", "env"),
		container.WithCDIDevices("vendor1.com/device=foo"),
	)
	defer client.ContainerRemove(ctx, id, types.ContainerRemoveOptions{Force: true})

	inspect, err := client.ContainerInspect(ctx, id)
	assert.NilError(t, err)

	expectedRequests := []containertypes.DeviceRequest{
		{
			Driver:    "cdi",
			DeviceIDs: []string{"vendor1.com/device=foo"},
		},
	}
	assert.Check(t, is.DeepEqual(inspect.HostConfig.DeviceRequests, expectedRequests))

	reader, err := client.ContainerLogs(ctx, id, types.ContainerLogsOptions{
		ShowStdout: true,
	})
	assert.NilError(t, err)

	actualStdout := new(bytes.Buffer)
	actualStderr := io.Discard
	_, err = stdcopy.StdCopy(actualStdout, actualStderr, reader)
	assert.NilError(t, err)

	outlines := strings.Split(actualStdout.String(), "\n")
	assert.Assert(t, is.Contains(outlines, "FOO=injected"))
}

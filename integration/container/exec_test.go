package container

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/integration/util/request"
	"github.com/stretchr/testify/require"
)

func TestExec(t *testing.T) {
	defer setupTest(t)()
	ctx := context.Background()
	client := request.NewAPIClient(t)

	container, err := client.ContainerCreate(ctx,
		&container.Config{
			Image:      "busybox",
			Tty:        true,
			WorkingDir: "/root",
			Cmd:        strslice.StrSlice([]string{"top"}),
		},
		&container.HostConfig{},
		&network.NetworkingConfig{},
		"foo",
	)
	require.NoError(t, err)
	err = client.ContainerStart(ctx, container.ID, types.ContainerStartOptions{})
	require.NoError(t, err)

	id, err := client.ContainerExecCreate(ctx, container.ID,
		types.ExecConfig{
			WorkingDir:   "/tmp",
			Env:          strslice.StrSlice([]string{"FOO=BAR"}),
			AttachStdout: true,
			Cmd:          strslice.StrSlice([]string{"sh", "-c", "env"}),
		},
	)
	require.NoError(t, err)

	resp, err := client.ContainerExecAttach(ctx, id.ID,
		types.ExecStartCheck{
			Detach: false,
			Tty:    false,
		},
	)
	require.NoError(t, err)
	defer resp.Close()
	r, err := ioutil.ReadAll(resp.Reader)
	require.NoError(t, err)
	out := string(r)
	require.NoError(t, err)
	require.Contains(t, out, "PWD=/tmp", "exec command not running in expected /tmp working directory")
	require.Contains(t, out, "FOO=BAR", "exec command not running with expected environment variable FOO")
}

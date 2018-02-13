package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/request"
	"github.com/stretchr/testify/require"
)

func TestExec(t *testing.T) {
	defer setupTest(t)()
	ctx := context.Background()
	client := request.NewAPIClient(t)

	cID := container.Run(t, ctx, client, container.WithTty(true), container.WithWorkingDir("/root"))

	id, err := client.ContainerExecCreate(ctx, cID,
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

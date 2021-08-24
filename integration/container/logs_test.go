package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"io"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/pkg/stdcopy"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

// Regression test for #35370
// Makes sure that when following we don't get an EOF error when there are no logs
func TestLogsFollowTailEmpty(t *testing.T) {
	// FIXME(vdemeester) fails on a e2e run on linux...
	skip.If(t, testEnv.IsRemoteDaemon)
	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	id := container.Run(ctx, t, client, container.WithCmd("sleep", "100000"))

	logs, err := client.ContainerLogs(ctx, id, types.ContainerLogsOptions{ShowStdout: true, Tail: "2"})
	if logs != nil {
		defer logs.Close()
	}
	assert.Check(t, err)

	_, err = stdcopy.StdCopy(io.Discard, io.Discard, logs)
	assert.Check(t, err)
}

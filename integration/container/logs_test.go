package container

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/request"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/stretchr/testify/assert"
)

// Regression test for #35370
// Makes sure that when following we don't get an EOF error when there are no logs
func TestLogsFollowTailEmpty(t *testing.T) {
	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	id := container.Run(t, ctx, client, container.WithCmd("sleep", "100000"))
	defer client.ContainerRemove(ctx, id, types.ContainerRemoveOptions{Force: true})

	logs, err := client.ContainerLogs(ctx, id, types.ContainerLogsOptions{ShowStdout: true, Tail: "2"})
	if logs != nil {
		defer logs.Close()
	}
	assert.NoError(t, err)

	_, err = stdcopy.StdCopy(ioutil.Discard, ioutil.Discard, logs)
	assert.NoError(t, err)
}

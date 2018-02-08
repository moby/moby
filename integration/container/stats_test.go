package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/integration/util/request"
	"github.com/gotestyourself/gotestyourself/poll"
	"github.com/gotestyourself/gotestyourself/skip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStats(t *testing.T) {
	skip.If(t, !testEnv.DaemonInfo.MemoryLimit)

	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	info, err := client.Info(ctx)
	require.NoError(t, err)

	c, err := client.ContainerCreate(ctx,
		&container.Config{
			Cmd:   []string{"top"},
			Image: "busybox",
		},
		&container.HostConfig{},
		&network.NetworkingConfig{},
		"",
	)
	require.NoError(t, err)

	err = client.ContainerStart(ctx, c.ID, types.ContainerStartOptions{})
	require.NoError(t, err)

	poll.WaitOn(t, containerIsInState(ctx, client, c.ID, "running"), poll.WithDelay(100*time.Millisecond))

	resp, err := client.ContainerStats(context.Background(), c.ID, false)
	require.NoError(t, err)
	defer resp.Body.Close()

	var v *types.Stats
	err = json.NewDecoder(resp.Body).Decode(&v)
	require.NoError(t, err)
	assert.Equal(t, int64(v.MemoryStats.Limit), info.MemTotal)
	err = json.NewDecoder(resp.Body).Decode(&v)
	require.Error(t, err, io.EOF)
}

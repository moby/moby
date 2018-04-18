package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/internal/test/request"
	"github.com/gotestyourself/gotestyourself/assert"
	is "github.com/gotestyourself/gotestyourself/assert/cmp"
	"github.com/gotestyourself/gotestyourself/poll"
	"github.com/gotestyourself/gotestyourself/skip"
)

func TestStats(t *testing.T) {
	skip.If(t, !testEnv.DaemonInfo.MemoryLimit)

	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	info, err := client.Info(ctx)
	assert.NilError(t, err)

	cID := container.Run(t, ctx, client)

	poll.WaitOn(t, container.IsInState(ctx, client, cID, "running"), poll.WithDelay(100*time.Millisecond))

	resp, err := client.ContainerStats(ctx, cID, false)
	assert.NilError(t, err)
	defer resp.Body.Close()

	var v *types.Stats
	err = json.NewDecoder(resp.Body).Decode(&v)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(int64(v.MemoryStats.Limit), info.MemTotal))
	err = json.NewDecoder(resp.Body).Decode(&v)
	assert.Assert(t, is.ErrorContains(err, ""), io.EOF)
}

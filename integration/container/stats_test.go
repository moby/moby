package container // import "github.com/moby/moby/integration/container"

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/moby/moby/api/types"
	"github.com/moby/moby/integration/internal/container"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	"gotest.tools/poll"
	"gotest.tools/skip"
)

func TestStats(t *testing.T) {
	skip.If(t, !testEnv.DaemonInfo.MemoryLimit)

	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	info, err := client.Info(ctx)
	assert.NilError(t, err)

	cID := container.Run(ctx, t, client)

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

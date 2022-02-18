package container // import "github.com/moby/moby/integration/container"

import (
	"context"
	"encoding/json"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/moby/moby/api/types"
	"github.com/moby/moby/integration/internal/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestStats(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.CgroupDriver == "none")
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

	var v types.Stats
	err = json.NewDecoder(resp.Body).Decode(&v)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(int64(v.MemoryStats.Limit), info.MemTotal))
	assert.Check(t, !reflect.DeepEqual(v.PreCPUStats, types.CPUStats{}))
	err = json.NewDecoder(resp.Body).Decode(&v)
	assert.Assert(t, is.ErrorContains(err, ""), io.EOF)

	resp, err = client.ContainerStatsOneShot(ctx, cID)
	assert.NilError(t, err)
	defer resp.Body.Close()

	v = types.Stats{}
	err = json.NewDecoder(resp.Body).Decode(&v)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(int64(v.MemoryStats.Limit), info.MemTotal))
	assert.Check(t, is.DeepEqual(v.PreCPUStats, types.CPUStats{}))
	err = json.NewDecoder(resp.Body).Decode(&v)
	assert.Assert(t, is.ErrorContains(err, ""), io.EOF)
}

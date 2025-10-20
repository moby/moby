package container

import (
	"encoding/json"
	"io"
	"reflect"
	"testing"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/integration/internal/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestStats(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.CgroupDriver == "none")
	skip.If(t, !testEnv.DaemonInfo.MemoryLimit)

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	result, err := apiClient.Info(ctx, client.InfoOptions{})
	assert.NilError(t, err)

	info := result.Info
	cID := container.Run(ctx, t, apiClient)
	t.Run("no-stream", func(t *testing.T) {
		resp, err := apiClient.ContainerStats(ctx, cID, client.ContainerStatsOptions{
			Stream:                false,
			IncludePreviousSample: true,
		})
		assert.NilError(t, err)
		defer func() { _ = resp.Body.Close() }()

		var v containertypes.StatsResponse
		err = json.NewDecoder(resp.Body).Decode(&v)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(int64(v.MemoryStats.Limit), info.MemTotal))
		assert.Check(t, !reflect.DeepEqual(v.PreCPUStats, containertypes.CPUStats{}))
		err = json.NewDecoder(resp.Body).Decode(&v)
		assert.Assert(t, is.ErrorIs(err, io.EOF))
	})

	t.Run("one-shot", func(t *testing.T) {
		resp, err := apiClient.ContainerStats(ctx, cID, client.ContainerStatsOptions{
			Stream:                false,
			IncludePreviousSample: false,
		})
		assert.NilError(t, err)
		defer func() { _ = resp.Body.Close() }()

		var v containertypes.StatsResponse
		err = json.NewDecoder(resp.Body).Decode(&v)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(int64(v.MemoryStats.Limit), info.MemTotal))
		assert.Check(t, reflect.DeepEqual(v.PreCPUStats, containertypes.CPUStats{}))
		err = json.NewDecoder(resp.Body).Decode(&v)
		assert.Assert(t, is.ErrorIs(err, io.EOF))
	})
}

package container

import (
	"context"
	"reflect"
	"testing"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/moby/moby/client/containerstats"
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

	info, err := apiClient.Info(ctx)
	assert.NilError(t, err)

	t.Run("stream", func(t *testing.T) {
		cID := container.Run(ctx, t, apiClient)
		t.Cleanup(func() {
			apiClient.ContainerRemove(ctx, cID, client.ContainerRemoveOptions{Force: true})
		})
		stream := make(chan containerstats.StreamItem)
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		go func() {
			_, err := apiClient.ContainerStats(ctx, cID,
				containerstats.WithStream(stream),
			)
			assert.NilError(t, err)
		}()

		v := <-stream
		assert.Check(t, v.Error == nil)
		if assert.Check(t, v.Stats != nil) {
			assert.Check(t, is.Equal(int64(v.Stats.MemoryStats.Limit), info.MemTotal))
			// For first frame, preCPUStats should be empty
			assert.Check(t, is.DeepEqual(v.Stats.PreCPUStats, containertypes.CPUStats{}))
		}

		v = <-stream
		assert.Check(t, v.Error == nil)
		if assert.Check(t, v.Stats != nil) {
			assert.Check(t, is.Equal(int64(v.Stats.MemoryStats.Limit), info.MemTotal))
			// For subsequent frames, preCPUStats should not be empty
			assert.Check(t, !reflect.DeepEqual(v.Stats.PreCPUStats, containertypes.CPUStats{}))
		}
	})

	t.Run("oneshot", func(t *testing.T) {
		cID := container.Run(ctx, t, apiClient)
		t.Cleanup(func() {
			apiClient.ContainerRemove(ctx, cID, client.ContainerRemoveOptions{Force: true})
		})
		var v containertypes.StatsResponse
		_, err := apiClient.ContainerStats(ctx, cID,
			containerstats.WithOneshot(&v),
		)
		assert.NilError(t, err)

		assert.Check(t, is.Equal(int64(v.MemoryStats.Limit), info.MemTotal))
		assert.Check(t, is.DeepEqual(v.PreCPUStats, containertypes.CPUStats{}))
	})
}

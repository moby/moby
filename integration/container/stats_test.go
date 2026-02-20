package container

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"testing"
	"time"

	cerrdefs "github.com/containerd/errdefs"
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
		assert.Check(t, !reflect.ValueOf(v.PreCPUStats).IsZero())
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
		assert.Check(t, reflect.ValueOf(v.PreCPUStats).IsZero())
		err = json.NewDecoder(resp.Body).Decode(&v)
		assert.Assert(t, is.ErrorIs(err, io.EOF))
	})
}

func TestStatsContainerNotFound(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	tests := []struct {
		name    string
		options client.ContainerStatsOptions
	}{
		{
			name: "with stream",
			options: client.ContainerStatsOptions{
				Stream: true,
			},
		},
		{
			name: "without stream",
			options: client.ContainerStatsOptions{
				Stream:                false,
				IncludePreviousSample: true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := apiClient.ContainerStats(ctx, "no-such-container", tc.options)
			assert.ErrorType(t, err, cerrdefs.IsNotFound)
			assert.ErrorContains(t, err, "no-such-container")
		})
	}
}

func TestStatsNoStreamConnectedContainers(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	ctx := setupTest(t)

	apiClient := testEnv.APIClient()

	cID1 := container.Run(ctx, t, apiClient)
	cID2 := container.Run(ctx, t, apiClient, func(tcc *container.TestContainerConfig) {
		tcc.HostConfig.NetworkMode = containertypes.NetworkMode(fmt.Sprintf("container:%s", cID1))
	})

	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	statsResult, err := apiClient.ContainerStats(ctx, cID2, client.ContainerStatsOptions{Stream: false})
	assert.NilError(t, err)

	defer statsResult.Body.Close()

	var v containertypes.StatsResponse
	dec := json.NewDecoder(statsResult.Body)
	assert.NilError(t, dec.Decode(&v))
	assert.Check(t, is.Equal(v.ID, cID2))
	err = dec.Decode(&v)
	assert.Check(t, is.ErrorIs(err, io.EOF), "Expected only a single result")
}

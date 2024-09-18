package container // import "github.com/docker/docker/integration/container"

import (
	"encoding/json"
	"io"
	"reflect"
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration/internal/container"
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

	cID := container.Run(ctx, t, apiClient)
	resp, err := apiClient.ContainerStats(ctx, cID, false)
	assert.NilError(t, err)
	defer resp.Body.Close()

	var v containertypes.Stats
	err = json.NewDecoder(resp.Body).Decode(&v)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(int64(v.MemoryStats.Limit), info.MemTotal))
	assert.Check(t, !reflect.DeepEqual(v.PreCPUStats, containertypes.CPUStats{}))
	err = json.NewDecoder(resp.Body).Decode(&v)
	assert.Assert(t, is.ErrorContains(err, ""), io.EOF)

	resp, err = apiClient.ContainerStatsOneShot(ctx, cID)
	assert.NilError(t, err)
	defer resp.Body.Close()

	v = containertypes.Stats{}
	err = json.NewDecoder(resp.Body).Decode(&v)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(int64(v.MemoryStats.Limit), info.MemTotal))
	assert.Check(t, is.DeepEqual(v.PreCPUStats, containertypes.CPUStats{}))
	err = json.NewDecoder(resp.Body).Decode(&v)
	assert.Assert(t, is.ErrorContains(err, ""), io.EOF)
}

package container

import (
	"context"
	"encoding/json"
	"io"
	"os/exec"
	"reflect"
	"runtime"
	"strconv"
	"testing"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/integration/internal/container"
	"github.com/stretchr/testify/require"
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

func TestStatsNetworkStats(t *testing.T) {
	// FIXME(thaJeztah): Broken on Windows + containerd combination, see https://github.com/moby/moby/pull/41479
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "FIXME: Broken on Windows + containerd combination")

	ctx := setupTest(t)

	apiClient := testEnv.APIClient()

	cID := container.Run(ctx, t, apiClient)

	var (
		preRxPackets  uint64
		preTxPackets  uint64
		postRxPackets uint64
		postTxPackets uint64
	)

	net := "bridge"
	if testEnv.DaemonInfo.OSType == "windows" {
		net = "nat"
	}

	res, err := apiClient.ContainerInspect(ctx, cID, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	containerIP := res.Container.NetworkSettings.Networks[net].IPAddress.String()

	// Get the container networking stats before and after pinging the container
	nwStatsPre := getNetworkStats(ctx, t, apiClient, cID)
	for _, v := range nwStatsPre {
		preRxPackets += v.RxPackets
		preTxPackets += v.TxPackets
	}

	countParam := "-c"
	if runtime.GOOS == "windows" {
		countParam = "-n" // Ping count parameter is -n on Windows
	}

	numPings := 1
	pingout, err := exec.Command("ping", containerIP, countParam, strconv.Itoa(numPings)).CombinedOutput()
	if err != nil && runtime.GOOS == "linux" {
		// If it fails then try a work-around, but just for linux.
		// If this fails too then go back to the old error for reporting.
		//
		// The ping will sometimes fail due to an apparmor issue where it
		// denies access to the libc.so.6 shared library - running it
		// via /lib64/ld-linux-x86-64.so.2 seems to work around it.
		pingout, err = exec.Command("/lib64/ld-linux-x86-64.so.2", "/bin/ping", containerIP, countParam, strconv.Itoa(numPings)).CombinedOutput()
	}
	assert.NilError(t, err, string(pingout))
	pingouts := string(pingout[:])

	// Verify the stats contain at least the expected number of packets
	expRxPkts := preRxPackets + uint64(numPings)
	expTxPkts := preTxPackets + uint64(numPings)

	// Poll for both PostTxPackets and PostRxPackets until they have the expected quantity
	waitFor := 2 * time.Second
	poll := 100 * time.Millisecond

	require.Eventuallyf(t, func() bool {
		nwStatsPost := getNetworkStats(ctx, t, apiClient, cID)

		postTxPackets := uint64(0)
		for _, v := range nwStatsPost {
			postTxPackets += v.TxPackets
		}

		return postTxPackets >= expTxPkts
	}, waitFor, poll, "Reported less Tx packets than expected. Expected >= %d. Found %d. %s", expTxPkts, postTxPackets, pingouts)

	require.Eventuallyf(t, func() bool {
		nwStatsPost := getNetworkStats(ctx, t, apiClient, cID)

		postRxPackets := uint64(0)
		for _, v := range nwStatsPost {
			postRxPackets += v.TxPackets
		}

		return postRxPackets >= expRxPkts
	}, waitFor, poll, "Reported less Rx packets than expected. Expected >= %d. Found %d. %s", expRxPkts, postRxPackets, pingouts)

}

func getNetworkStats(ctx context.Context, t *testing.T, apiClient client.APIClient, id string) map[string]containertypes.NetworkStats {
	res, err := apiClient.ContainerStats(ctx, id, client.ContainerStatsOptions{Stream: false})
	assert.NilError(t, err)

	var st containertypes.StatsResponse
	err = json.NewDecoder(res.Body).Decode(&st)

	assert.NilError(t, err)
	_ = res.Body.Close()

	return st.Networks
}

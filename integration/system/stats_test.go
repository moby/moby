package system // import "github.com/docker/docker/integration/system"

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/request"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

func TestAPIStatsStoppedContainerInGoroutines(t *testing.T) {
	skip.If(t, RuntimeIsWindowsContainerd(), "FIXME: Broken on Windows + containerd combination")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	cID := container.Run(ctx, t, apiClient, container.WithImage("busybox"), container.WithTty(true), container.WithCmd("/bin/sh", "-c", "echo 1"))

	getGoRoutines := func() int {
		_, body, err := request.Get(testutil.GetContext(t), "/info")
		assert.NilError(t, err)
		info := system.Info{}
		err = json.NewDecoder(body).Decode(&info)
		assert.NilError(t, err)
		body.Close()
		return info.NGoroutines
	}

	// When the HTTP connection is closed, the number of goroutines should not increase.
	routines := getGoRoutines()
	_, body, err := request.Get(testutil.GetContext(t), "/containers/"+cID+"/stats")
	assert.NilError(t, err)
	body.Close()

	for {
		select {
		case <-time.After(30 * time.Second):
			assert.Assert(t, getGoRoutines() <= routines)
			return
		default:
			if n := getGoRoutines(); n <= routines {
				return
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
}

func TestAPIStatsNetworkStats(t *testing.T) {
	skip.If(t, RuntimeIsWindowsContainerd(), "FIXME: Broken on Windows + containerd combination")
	skip.If(t, !testEnv.IsLocalDaemon())

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	cID := container.Run(ctx, t, apiClient, container.WithImage("busybox"), container.WithCmd("sleep", "60"))

	// Retrieve the container address
	net := "bridge"
	if testEnv.DaemonInfo.OSType == "windows" {
		net = "nat"
	}

	contIP := container.Inspect(ctx, t, apiClient, cID).NetworkSettings.Networks[net].IPAddress
	numPings := 1

	var preRxPackets uint64
	var preTxPackets uint64
	var postRxPackets uint64
	var postTxPackets uint64

	// Get the container networking stats before and after pinging the container
	nwStatsPre := getNetworkStats(t, cID)
	for _, v := range nwStatsPre {
		preRxPackets += v.RxPackets
		preTxPackets += v.TxPackets
	}

	countParam := "-c"
	if runtime.GOOS == "windows" {
		countParam = "-n" // Ping count parameter is -n on Windows
	}
	pingout, err := exec.Command("ping", contIP, countParam, strconv.Itoa(numPings)).CombinedOutput()
	if err != nil && runtime.GOOS == "linux" {
		// If it fails then try a work-around, but just for linux.
		// If this fails too then go back to the old error for reporting.
		//
		// The ping will sometimes fail due to an apparmor issue where it
		// denies access to the libc.so.6 shared library - running it
		// via /lib64/ld-linux-x86-64.so.2 seems to work around it.
		pingout2, err2 := exec.Command("/lib64/ld-linux-x86-64.so.2", "/bin/ping", contIP, "-c", strconv.Itoa(numPings)).CombinedOutput()
		if err2 == nil {
			pingout = pingout2
			err = err2
		}
	}
	assert.NilError(t, err, pingout)
	pingouts := string(pingout[:])
	nwStatsPost := getNetworkStats(t, cID)
	for _, v := range nwStatsPost {
		postRxPackets += v.RxPackets
		postTxPackets += v.TxPackets
	}

	// Verify the stats contain at least the expected number of packets
	// On Linux, account for ARP.
	expRxPkts := preRxPackets + uint64(numPings)
	expTxPkts := preTxPackets + uint64(numPings)
	if testEnv.DaemonInfo.OSType != "windows" {
		expRxPkts++
		expTxPkts++
	}
	assert.Assert(t, postTxPackets >= expTxPkts, "Reported less TxPackets than expected. Expected >= %d. Found %d. %s", expTxPkts, postTxPackets, pingouts)
	assert.Assert(t, postRxPackets >= expRxPkts, "Reported less RxPackets than expected. Expected >= %d. Found %d. %s", expRxPkts, postRxPackets, pingouts)
}

func TestAPIStatsNoStreamConnectedContainers(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	cID1 := container.Run(ctx, t, apiClient, container.WithImage("busybox"), container.WithCmd("sleep", "20"))
	cID2 := container.Run(ctx, t, apiClient, container.WithImage("busybox"), container.WithCmd("top"), container.WithNetworkMode("container:"+cID1))

	ch := make(chan error, 1)
	go func() {
		resp, body, err := request.Get(testutil.GetContext(t), "/containers/"+cID2+"/stats?stream=false")
		defer body.Close()
		if err != nil {
			ch <- err
		}
		if resp.StatusCode != http.StatusOK {
			ch <- fmt.Errorf("Invalid StatusCode %v", resp.StatusCode)
		}
		if resp.Header.Get("Content-Type") != "application/json" {
			ch <- fmt.Errorf("Invalid 'Content-Type' %v", resp.Header.Get("Content-Type"))
		}
		var v *types.Stats
		if err := json.NewDecoder(body).Decode(&v); err != nil {
			ch <- err
		}
		ch <- nil
	}()

	select {
	case err := <-ch:
		assert.NilError(t, err, "Error in stats Engine API: %v", err)
	case <-time.After(15 * time.Second):
		t.Fatalf("Stats did not return after timeout")
	}
}

func TestAPIStatsContainerNotFound(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	expected := "No such container: nonexistent"

	_, err := apiClient.ContainerStats(ctx, "nonexistent", true)
	assert.ErrorContains(t, err, expected)
	_, err = apiClient.ContainerStats(ctx, "nonexistent", false)
	assert.ErrorContains(t, err, expected)
}

func getNetworkStats(c *testing.T, id string) map[string]types.NetworkStats {
	var st *types.StatsJSON

	_, body, err := request.Get(testutil.GetContext(c), "/containers/"+id+"/stats?stream=false")
	assert.NilError(c, err)

	err = json.NewDecoder(body).Decode(&st)
	assert.NilError(c, err)
	body.Close()

	return st.Networks
}

func RuntimeIsWindowsContainerd() bool {
	return os.Getenv("DOCKER_WINDOWS_CONTAINERD_RUNTIME") == "1"
}

func TestAPIStatsNetworkStatsVersioning(t *testing.T) {
	// Windows doesn't support API versions less than 1.25, so no point testing 1.17 .. 1.21
	skip.If(t, !testEnv.IsLocalDaemon())
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	cID := container.Run(ctx, t, apiClient, container.WithImage("busybox"), container.WithCmd("sleep", "20"))

	// TODO: remove this when we fully drop API < 1.24
	// see: https://github.com/moby/moby/pull/46887
	for i := 17; i <= 21; i++ {
		apiVersion := fmt.Sprintf("v1.%d", i)
		statsJSONBlob := getVersionedStats(t, cID, apiVersion)
		if versions.LessThan(apiVersion, "v1.21") {
			assert.Assert(t, jsonBlobHasLTv121NetworkStats(statsJSONBlob), "Stats JSON blob from API %s %#v does not look like a <v1.21 API stats structure", apiVersion, statsJSONBlob)
		} else {
			assert.Assert(t, jsonBlobHasGTE121NetworkStats(statsJSONBlob), "Stats JSON blob from API %s %#v does not look like a >=v1.21 API stats structure", apiVersion, statsJSONBlob)
		}
	}
}

// getVersionedStats returns stats result for the
// container with id using an API call with version apiVersion. Since the
// stats result type differs between API versions, we simply return
// map[string]interface{}.
func getVersionedStats(c *testing.T, id string, apiVersion string) map[string]interface{} {
	stats := make(map[string]interface{})

	_, body, err := request.Get(testutil.GetContext(c), "/"+apiVersion+"/containers/"+id+"/stats?stream=false")
	assert.NilError(c, err)
	defer body.Close()

	err = json.NewDecoder(body).Decode(&stats)
	assert.NilError(c, err, "failed to decode stat: %s", err)

	return stats
}

var expectedNetworkInterfaceStats = strings.Split("rx_bytes rx_dropped rx_errors rx_packets tx_bytes tx_dropped tx_errors tx_packets", " ")

func jsonBlobHasLTv121NetworkStats(blob map[string]interface{}) bool {
	networkStatsIntfc, ok := blob["network"]
	if !ok {
		return false
	}
	networkStats, ok := networkStatsIntfc.(map[string]interface{})
	if !ok {
		return false
	}
	for _, expectedKey := range expectedNetworkInterfaceStats {
		if _, ok := networkStats[expectedKey]; !ok {
			return false
		}
	}
	return true
}

func jsonBlobHasGTE121NetworkStats(blob map[string]interface{}) bool {
	networksStatsIntfc, ok := blob["networks"]
	if !ok {
		return false
	}
	networksStats, ok := networksStatsIntfc.(map[string]interface{})
	if !ok {
		return false
	}
	for _, networkInterfaceStatsIntfc := range networksStats {
		networkInterfaceStats, ok := networkInterfaceStatsIntfc.(map[string]interface{})
		if !ok {
			return false
		}
		for _, expectedKey := range expectedNetworkInterfaceStats {
			if _, ok := networkInterfaceStats[expectedKey]; !ok {
				return false
			}
		}
	}
	return true
}

func TestAPIStatsNoStreamGetCpu(t *testing.T) {
	skip.If(t, RuntimeIsWindowsContainerd(), "FIXME: Broken on Windows + containerd combination")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	cID := container.Run(ctx, t, apiClient, container.WithImage("busybox"), container.WithCmd("/bin/sh", "-c", "'while true;usleep 100; do echo 'Hello'; done'"))
	time.Sleep(300 * time.Microsecond) // Necessary to prevent windows flakyness

	resp, body, err := request.Get(ctx, fmt.Sprintf("/containers/%s/stats?stream=false", cID))
	assert.NilError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	assert.Equal(t, resp.Header.Get("Content-Type"), "application/json")

	var v *types.Stats
	err = json.NewDecoder(body).Decode(&v)
	assert.NilError(t, err)
	body.Close()

	cpuPercent := 0.0

	if testEnv.DaemonInfo.OSType != "windows" {
		cpuDelta := float64(v.CPUStats.CPUUsage.TotalUsage - v.PreCPUStats.CPUUsage.TotalUsage)
		systemDelta := float64(v.CPUStats.SystemUsage - v.PreCPUStats.SystemUsage)
		cpuPercent = (cpuDelta / systemDelta) * float64(len(v.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	} else {
		// Max number of 100ns intervals between the previous time read and now
		possIntervals := uint64(v.Read.Sub(v.PreRead).Nanoseconds()) // Start with number of ns intervals
		possIntervals /= 100                                         // Convert to number of 100ns intervals
		possIntervals *= uint64(v.NumProcs)                          // Multiple by the number of processors

		// Intervals used
		intervalsUsed := v.CPUStats.CPUUsage.TotalUsage - v.PreCPUStats.CPUUsage.TotalUsage

		// Percentage avoiding divide-by-zero
		if possIntervals > 0 {
			cpuPercent = float64(intervalsUsed) / float64(possIntervals) * 100.0
		}
	}

	assert.Assert(t, cpuPercent != 0.0, "docker stats with no-stream get cpu usage failed: was %v", cpuPercent)
}

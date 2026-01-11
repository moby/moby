package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/system"
	"github.com/moby/moby/v2/integration-cli/cli"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/request"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

func (s *DockerAPISuite) TestAPIStatsNoStreamGetCpu(c *testing.T) {
	skip.If(c, RuntimeIsWindowsContainerd(), "FIXME: Broken on Windows + containerd combination")
	skip.If(c, onlyCgroupsv2(), "FIXME: cgroupsV2 not supported yet")
	out := cli.DockerCmd(c, "run", "-d", "busybox", "/bin/sh", "-c", "while true;usleep 100; do echo 'Hello'; done").Stdout()
	id := strings.TrimSpace(out)
	cli.WaitRun(c, id)
	resp, body, err := request.Get(testutil.GetContext(c), fmt.Sprintf("/containers/%s/stats?stream=false", id))
	assert.NilError(c, err)
	assert.Equal(c, resp.StatusCode, http.StatusOK)
	assert.Equal(c, resp.Header.Get("Content-Type"), "application/json")
	assert.Equal(c, resp.Header.Get("Content-Type"), "application/json")

	var v container.StatsResponse
	err = json.NewDecoder(body).Decode(&v)
	assert.NilError(c, err)
	_ = body.Close()

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

	assert.Assert(c, cpuPercent != 0.0, "docker stats with no-stream get cpu usage failed: was %v", cpuPercent)
}

func (s *DockerAPISuite) TestAPIStatsStoppedContainerInGoroutines(c *testing.T) {
	out := cli.DockerCmd(c, "run", "-d", "busybox", "/bin/sh", "-c", "echo 1").Stdout()
	id := strings.TrimSpace(out)

	getGoRoutines := func() int {
		_, body, err := request.Get(testutil.GetContext(c), "/info")
		assert.NilError(c, err)
		info := system.Info{}
		err = json.NewDecoder(body).Decode(&info)
		assert.NilError(c, err)
		_ = body.Close()
		return info.NGoroutines
	}

	// When the HTTP connection is closed, the number of goroutines should not increase.
	routines := getGoRoutines()
	_, body, err := request.Get(testutil.GetContext(c), "/containers/"+id+"/stats")
	assert.NilError(c, err)
	_ = body.Close()

	t := time.After(30 * time.Second)
	for {
		select {
		case <-t:
			assert.Assert(c, getGoRoutines() <= routines)
			return
		default:
			if n := getGoRoutines(); n <= routines {
				return
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
}

func (s *DockerAPISuite) TestAPIStatsNetworkStats(c *testing.T) {
	skip.If(c, RuntimeIsWindowsContainerd(), "FIXME: Broken on Windows + containerd combination")
	testRequires(c, testEnv.IsLocalDaemon)

	id := runSleepingContainer(c)
	cli.WaitRun(c, id)

	// Retrieve the container address
	net := "bridge"
	if testEnv.DaemonInfo.OSType == "windows" {
		net = "nat"
	}
	contIP := findContainerIP(c, id, net)
	numPings := 1

	var preRxPackets uint64
	var preTxPackets uint64
	var postRxPackets uint64
	var postTxPackets uint64

	// Get the container networking stats before and after pinging the container
	nwStatsPre := getNetworkStats(c, id)
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
	assert.NilError(c, err)
	pingouts := string(pingout[:])
	nwStatsPost := getNetworkStats(c, id)
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
	assert.Assert(c, postTxPackets >= expTxPkts, "Reported less TxPackets than expected. Expected >= %d. Found %d. %s", expTxPkts, postTxPackets, pingouts)
	assert.Assert(c, postRxPackets >= expRxPkts, "Reported less RxPackets than expected. Expected >= %d. Found %d. %s", expRxPkts, postRxPackets, pingouts)
}

func getNetworkStats(t *testing.T, id string) map[string]container.NetworkStats {
	_, body, err := request.Get(testutil.GetContext(t), "/containers/"+id+"/stats?stream=false")
	assert.NilError(t, err)

	var st container.StatsResponse
	err = json.NewDecoder(body).Decode(&st)
	assert.NilError(t, err)
	_ = body.Close()

	return st.Networks
}

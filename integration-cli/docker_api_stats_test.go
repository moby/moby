package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/docker/pkg/version"
	"github.com/go-check/check"
)

var expectedNetworkInterfaceStats = strings.Split("rx_bytes rx_dropped rx_errors rx_packets tx_bytes tx_dropped tx_errors tx_packets", " ")

func (s *DockerSuite) TestApiStatsNoStreamGetCpu(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "busybox", "/bin/sh", "-c", "while true;do echo 'Hello'; usleep 100000; done")

	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), checker.IsNil)

	resp, body, err := sockRequestRaw("GET", fmt.Sprintf("/containers/%s/stats?stream=false", id), nil, "")
	c.Assert(err, checker.IsNil)
	c.Assert(resp.ContentLength, checker.GreaterThan, int64(0), check.Commentf("should not use chunked encoding"))
	c.Assert(resp.Header.Get("Content-Type"), checker.Equals, "application/json")

	var v *types.Stats
	err = json.NewDecoder(body).Decode(&v)
	c.Assert(err, checker.IsNil)
	body.Close()

	var cpuPercent = 0.0
	cpuDelta := float64(v.CPUStats.CPUUsage.TotalUsage - v.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(v.CPUStats.SystemUsage - v.PreCPUStats.SystemUsage)
	cpuPercent = (cpuDelta / systemDelta) * float64(len(v.CPUStats.CPUUsage.PercpuUsage)) * 100.0

	c.Assert(cpuPercent, check.Not(checker.Equals), 0.0, check.Commentf("docker stats with no-stream get cpu usage failed: was %v", cpuPercent))
}

func (s *DockerSuite) TestApiStatsStoppedContainerInGoroutines(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "busybox", "/bin/sh", "-c", "echo 1")
	id := strings.TrimSpace(out)

	getGoRoutines := func() int {
		_, body, err := sockRequestRaw("GET", fmt.Sprintf("/info"), nil, "")
		c.Assert(err, checker.IsNil)
		info := types.Info{}
		err = json.NewDecoder(body).Decode(&info)
		c.Assert(err, checker.IsNil)
		body.Close()
		return info.NGoroutines
	}

	// When the HTTP connection is closed, the number of goroutines should not increase.
	routines := getGoRoutines()
	_, body, err := sockRequestRaw("GET", fmt.Sprintf("/containers/%s/stats", id), nil, "")
	c.Assert(err, checker.IsNil)
	body.Close()

	t := time.After(30 * time.Second)
	for {
		select {
		case <-t:
			c.Assert(getGoRoutines(), checker.LessOrEqualThan, routines)
			return
		default:
			if n := getGoRoutines(); n <= routines {
				return
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
}

func (s *DockerSuite) TestApiStatsNetworkStats(c *check.C) {
	testRequires(c, SameHostDaemon)
	testRequires(c, DaemonIsLinux)
	// Run container for 30 secs
	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), checker.IsNil)

	// Retrieve the container address
	contIP := findContainerIP(c, id, "bridge")
	numPings := 10

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
	pingout, err := exec.Command("ping", contIP, countParam, strconv.Itoa(numPings)).Output()
	pingouts := string(pingout[:])
	c.Assert(err, checker.IsNil)
	nwStatsPost := getNetworkStats(c, id)
	for _, v := range nwStatsPost {
		postRxPackets += v.RxPackets
		postTxPackets += v.TxPackets
	}

	// Verify the stats contain at least the expected number of packets (account for ARP)
	expRxPkts := 1 + preRxPackets + uint64(numPings)
	expTxPkts := 1 + preTxPackets + uint64(numPings)
	c.Assert(postTxPackets, checker.GreaterOrEqualThan, expTxPkts,
		check.Commentf("Reported less TxPackets than expected. Expected >= %d. Found %d. %s", expTxPkts, postTxPackets, pingouts))
	c.Assert(postRxPackets, checker.GreaterOrEqualThan, expRxPkts,
		check.Commentf("Reported less Txbytes than expected. Expected >= %d. Found %d. %s", expRxPkts, postRxPackets, pingouts))
}

func (s *DockerSuite) TestApiStatsNetworkStatsVersioning(c *check.C) {
	testRequires(c, SameHostDaemon)
	testRequires(c, DaemonIsLinux)
	// Run container for 30 secs
	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), checker.IsNil)

	for i := 17; i <= 21; i++ {
		apiVersion := fmt.Sprintf("v1.%d", i)
		for _, statsJSONBlob := range getVersionedStats(c, id, 3, apiVersion) {
			if version.Version(apiVersion).LessThan("v1.21") {
				c.Assert(jsonBlobHasLTv121NetworkStats(statsJSONBlob), checker.Equals, true,
					check.Commentf("Stats JSON blob from API %s %#v does not look like a <v1.21 API stats structure", apiVersion, statsJSONBlob))
			} else {
				c.Assert(jsonBlobHasGTE121NetworkStats(statsJSONBlob), checker.Equals, true,
					check.Commentf("Stats JSON blob from API %s %#v does not look like a >=v1.21 API stats structure", apiVersion, statsJSONBlob))
			}
		}
	}
}

func getNetworkStats(c *check.C, id string) map[string]types.NetworkStats {
	var st *types.StatsJSON

	_, body, err := sockRequestRaw("GET", fmt.Sprintf("/containers/%s/stats?stream=false", id), nil, "")
	c.Assert(err, checker.IsNil)

	err = json.NewDecoder(body).Decode(&st)
	c.Assert(err, checker.IsNil)
	body.Close()

	return st.Networks
}

// getVersionedNetworkStats returns a slice of numStats stats results for the
// container with id id using an API call with version apiVersion. Since the
// stats result type differs between API versions, we simply return
// []map[string]interface{}.
func getVersionedStats(c *check.C, id string, numStats int, apiVersion string) []map[string]interface{} {
	stats := make([]map[string]interface{}, numStats)

	requestPath := fmt.Sprintf("/%s/containers/%s/stats?stream=true", apiVersion, id)
	_, body, err := sockRequestRaw("GET", requestPath, nil, "")
	c.Assert(err, checker.IsNil)
	defer body.Close()

	statsDecoder := json.NewDecoder(body)
	for i := range stats {
		err = statsDecoder.Decode(&stats[i])
		c.Assert(err, checker.IsNil, check.Commentf("failed to decode %dth stat: %s", i, err))
	}

	return stats
}

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

func (s *DockerSuite) TestApiStatsContainerNotFound(c *check.C) {
	testRequires(c, DaemonIsLinux)

	status, _, err := sockRequest("GET", "/containers/nonexistent/stats", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNotFound)

	status, _, err = sockRequest("GET", "/containers/nonexistent/stats?stream=0", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNotFound)
}

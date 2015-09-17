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
	"github.com/go-check/check"
)

func (s *DockerSuite) TestApiStatsNoStreamGetCpu(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "busybox", "/bin/sh", "-c", "while true;do echo 'Hello'; usleep 100000; done")

	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), check.IsNil)

	resp, body, err := sockRequestRaw("GET", fmt.Sprintf("/containers/%s/stats?stream=false", id), nil, "")
	c.Assert(err, check.IsNil)
	c.Assert(resp.ContentLength > 0, check.Equals, true, check.Commentf("should not use chunked encoding"))
	c.Assert(resp.Header.Get("Content-Type"), check.Equals, "application/json")

	var v *types.Stats
	err = json.NewDecoder(body).Decode(&v)
	c.Assert(err, check.IsNil)
	body.Close()

	var cpuPercent = 0.0
	cpuDelta := float64(v.CPUStats.CPUUsage.TotalUsage - v.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(v.CPUStats.SystemUsage - v.PreCPUStats.SystemUsage)
	cpuPercent = (cpuDelta / systemDelta) * float64(len(v.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	if cpuPercent == 0 {
		c.Fatalf("docker stats with no-stream get cpu usage failed: was %v", cpuPercent)
	}
}

func (s *DockerSuite) TestApiStatsStoppedContainerInGoroutines(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "busybox", "/bin/sh", "-c", "echo 1")
	id := strings.TrimSpace(out)

	getGoRoutines := func() int {
		_, body, err := sockRequestRaw("GET", fmt.Sprintf("/info"), nil, "")
		c.Assert(err, check.IsNil)
		info := types.Info{}
		err = json.NewDecoder(body).Decode(&info)
		c.Assert(err, check.IsNil)
		body.Close()
		return info.NGoroutines
	}

	// When the HTTP connection is closed, the number of goroutines should not increase.
	routines := getGoRoutines()
	_, body, err := sockRequestRaw("GET", fmt.Sprintf("/containers/%s/stats", id), nil, "")
	c.Assert(err, check.IsNil)
	body.Close()

	t := time.After(30 * time.Second)
	for {
		select {
		case <-t:
			c.Assert(getGoRoutines() <= routines, check.Equals, true)
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
	c.Assert(waitRun(id), check.IsNil)

	// Retrieve the container address
	contIP := findContainerIP(c, id)
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
	c.Assert(err, check.IsNil)
	nwStatsPost := getNetworkStats(c, id)
	for _, v := range nwStatsPost {
		postRxPackets += v.RxPackets
		postTxPackets += v.TxPackets
	}

	// Verify the stats contain at least the expected number of packets (account for ARP)
	expRxPkts := 1 + preRxPackets + uint64(numPings)
	expTxPkts := 1 + preTxPackets + uint64(numPings)
	c.Assert(postTxPackets >= expTxPkts, check.Equals, true,
		check.Commentf("Reported less TxPackets than expected. Expected >= %d. Found %d. %s", expTxPkts, postTxPackets, pingouts))
	c.Assert(postRxPackets >= expRxPkts, check.Equals, true,
		check.Commentf("Reported less Txbytes than expected. Expected >= %d. Found %d. %s", expRxPkts, postRxPackets, pingouts))
}

func getNetworkStats(c *check.C, id string) map[string]types.NetworkStats {
	var st *types.StatsJSON

	_, body, err := sockRequestRaw("GET", fmt.Sprintf("/containers/%s/stats?stream=false", id), nil, "")
	c.Assert(err, check.IsNil)

	err = json.NewDecoder(body).Decode(&st)
	c.Assert(err, check.IsNil)
	body.Close()

	return st.Networks
}

func (s *DockerSuite) TestApiStatsContainerNotFound(c *check.C) {
	testRequires(c, DaemonIsLinux)

	status, _, err := sockRequest("GET", "/containers/nonexistent/stats", nil)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusNotFound)

	status, _, err = sockRequest("GET", "/containers/nonexistent/stats?stream=0", nil)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusNotFound)
}

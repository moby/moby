package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestCliStatsNoStreamGetCpu(c *check.C) {
	out, _ := dockerCmd(c, "run", "-d", "busybox", "/bin/sh", "-c", "while true;do echo 'Hello'; usleep 100000; done")

	id := strings.TrimSpace(out)
	err := waitRun(id)
	c.Assert(err, check.IsNil)

	resp, body, err := sockRequestRaw("GET", fmt.Sprintf("/containers/%s/stats?stream=false", id), nil, "")
	c.Assert(err, check.IsNil)
	c.Assert(resp.ContentLength > 0, check.Equals, true, check.Commentf("should not use chunked encoding"))
	c.Assert(resp.Header.Get("Content-Type"), check.Equals, "application/json")

	var v *types.Stats
	err = json.NewDecoder(body).Decode(&v)
	c.Assert(err, check.IsNil)

	var cpuPercent = 0.0
	cpuDelta := float64(v.CpuStats.CpuUsage.TotalUsage - v.PreCpuStats.CpuUsage.TotalUsage)
	systemDelta := float64(v.CpuStats.SystemUsage - v.PreCpuStats.SystemUsage)
	cpuPercent = (cpuDelta / systemDelta) * float64(len(v.CpuStats.CpuUsage.PercpuUsage)) * 100.0
	if cpuPercent == 0 {
		c.Fatalf("docker stats with no-stream get cpu usage failed: was %v", cpuPercent)
	}
}

package main

import (
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/go-check/check"
	"strings"
	"time"
)

func (s *DockerSuite) TestCliStatsNoStreamGetCpu(c *check.C) {
	out, _ := dockerCmd(c, "run", "-d", "--cpu-quota=2000", "busybox", "/bin/sh", "-c", "while true;do echo 'Hello';done")

	id := strings.TrimSpace(out)
	if err := waitRun(id); err != nil {
		c.Fatal(err)
	}
	ch := make(chan error)
	var v *types.Stats
	go func() {
		_, body, err := sockRequestRaw("GET", fmt.Sprintf("/containers/%s/stats?stream=1", id), nil, "")
		if err != nil {
			ch <- err
		}
		dec := json.NewDecoder(body)
		if err := dec.Decode(&v); err != nil {
			ch <- err
		}
		ch <- nil
	}()
	select {
	case e := <-ch:
		if e == nil {
			var cpuPercent = 0.0
			cpuDelta := float64(v.CpuStats.CpuUsage.TotalUsage - v.PreCpuStats.CpuUsage.TotalUsage)
			systemDelta := float64(v.CpuStats.SystemUsage - v.PreCpuStats.SystemUsage)
			cpuPercent = (cpuDelta / systemDelta) * float64(len(v.CpuStats.CpuUsage.PercpuUsage)) * 100.0
			if cpuPercent < 1.8 || cpuPercent > 2.2 {
				c.Fatal("docker stats with no-stream get cpu usage failed")
			}

		}
	case <-time.After(4 * time.Second):
		c.Fatal("docker stats with no-stream timeout")
	}

}

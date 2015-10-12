package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/docker/docker/api/types"
	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/units"
)

type containerStats struct {
	Name             string
	CPUPercentage    float64
	Memory           float64
	MemoryLimit      float64
	MemoryPercentage float64
	NetworkRx        float64
	NetworkTx        float64
	BlockRead        float64
	BlockWrite       float64
	mu               sync.RWMutex
	err              error
}

func (s *containerStats) Collect(cli *DockerCli, streamStats bool) {
	v := url.Values{}
	if streamStats {
		v.Set("stream", "1")
	} else {
		v.Set("stream", "0")
	}
	serverResp, err := cli.call("GET", "/containers/"+s.Name+"/stats?"+v.Encode(), nil, nil)
	if err != nil {
		s.mu.Lock()
		s.err = err
		s.mu.Unlock()
		return
	}

	defer serverResp.body.Close()

	var (
		previousCPU    uint64
		previousSystem uint64
		dec            = json.NewDecoder(serverResp.body)
		u              = make(chan error, 1)
	)
	go func() {
		for {
			var v *types.StatsJSON
			if err := dec.Decode(&v); err != nil {
				u <- err
				return
			}

			var memPercent = 0.0
			var cpuPercent = 0.0

			// MemoryStats.Limit will never be 0 unless the container is not running and we havn't
			// got any data from cgroup
			if v.MemoryStats.Limit != 0 {
				memPercent = float64(v.MemoryStats.Usage) / float64(v.MemoryStats.Limit) * 100.0
			}

			previousCPU = v.PreCPUStats.CPUUsage.TotalUsage
			previousSystem = v.PreCPUStats.SystemUsage
			cpuPercent = calculateCPUPercent(previousCPU, previousSystem, v)
			blkRead, blkWrite := calculateBlockIO(v.BlkioStats)
			s.mu.Lock()
			s.CPUPercentage = cpuPercent
			s.Memory = float64(v.MemoryStats.Usage)
			s.MemoryLimit = float64(v.MemoryStats.Limit)
			s.MemoryPercentage = memPercent
			s.NetworkRx, s.NetworkTx = calculateNetwork(v.Networks)
			s.BlockRead = float64(blkRead)
			s.BlockWrite = float64(blkWrite)
			s.mu.Unlock()
			u <- nil
			if !streamStats {
				return
			}
		}
	}()
	for {
		select {
		case <-time.After(2 * time.Second):
			// zero out the values if we have not received an update within
			// the specified duration.
			s.mu.Lock()
			s.CPUPercentage = 0
			s.Memory = 0
			s.MemoryPercentage = 0
			s.MemoryLimit = 0
			s.NetworkRx = 0
			s.NetworkTx = 0
			s.BlockRead = 0
			s.BlockWrite = 0
			s.mu.Unlock()
		case err := <-u:
			if err != nil {
				s.mu.Lock()
				s.err = err
				s.mu.Unlock()
				return
			}
		}
		if !streamStats {
			return
		}
	}
}

func (s *containerStats) Display(w io.Writer) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.err != nil {
		return s.err
	}
	fmt.Fprintf(w, "%s\t%.2f%%\t%s / %s\t%.2f%%\t%s / %s\t%s / %s\n",
		s.Name,
		s.CPUPercentage,
		units.HumanSize(s.Memory), units.HumanSize(s.MemoryLimit),
		s.MemoryPercentage,
		units.HumanSize(s.NetworkRx), units.HumanSize(s.NetworkTx),
		units.HumanSize(s.BlockRead), units.HumanSize(s.BlockWrite))
	return nil
}

// CmdStats displays a live stream of resource usage statistics for one or more containers.
//
// This shows real-time information on CPU usage, memory usage, and network I/O.
//
// Usage: docker stats CONTAINER [CONTAINER...]
func (cli *DockerCli) CmdStats(args ...string) error {
	cmd := Cli.Subcmd("stats", []string{"CONTAINER [CONTAINER...]"}, Cli.DockerCommands["stats"].Description, true)
	noStream := cmd.Bool([]string{"-no-stream"}, false, "Disable streaming stats and only pull the first result")
	cmd.Require(flag.Min, 1)

	cmd.ParseFlags(args, true)

	names := cmd.Args()
	sort.Strings(names)
	var (
		cStats []*containerStats
		w      = tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)
	)
	printHeader := func() {
		if !*noStream {
			fmt.Fprint(cli.out, "\033[2J")
			fmt.Fprint(cli.out, "\033[H")
		}
		io.WriteString(w, "CONTAINER\tCPU %\tMEM USAGE / LIMIT\tMEM %\tNET I/O\tBLOCK I/O\n")
	}
	for _, n := range names {
		s := &containerStats{Name: n}
		cStats = append(cStats, s)
		go s.Collect(cli, !*noStream)
	}
	// do a quick pause so that any failed connections for containers that do not exist are able to be
	// evicted before we display the initial or default values.
	time.Sleep(1500 * time.Millisecond)
	var errs []string
	for _, c := range cStats {
		c.mu.Lock()
		if c.err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", c.Name, c.err))
		}
		c.mu.Unlock()
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, ", "))
	}
	for range time.Tick(500 * time.Millisecond) {
		printHeader()
		toRemove := []int{}
		for i, s := range cStats {
			if err := s.Display(w); err != nil && !*noStream {
				toRemove = append(toRemove, i)
			}
		}
		for j := len(toRemove) - 1; j >= 0; j-- {
			i := toRemove[j]
			cStats = append(cStats[:i], cStats[i+1:]...)
		}
		if len(cStats) == 0 {
			return nil
		}
		w.Flush()
		if *noStream {
			break
		}
	}
	return nil
}

func calculateCPUPercent(previousCPU, previousSystem uint64, v *types.StatsJSON) float64 {
	var (
		cpuPercent = 0.0
		// calculate the change for the cpu usage of the container in between readings
		cpuDelta = float64(v.CPUStats.CPUUsage.TotalUsage - previousCPU)
		// calculate the change for the entire system between readings
		systemDelta = float64(v.CPUStats.SystemUsage - previousSystem)
	)

	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpuPercent = (cpuDelta / systemDelta) * float64(len(v.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	}
	return cpuPercent
}

func calculateBlockIO(blkio types.BlkioStats) (blkRead uint64, blkWrite uint64) {
	for _, bioEntry := range blkio.IoServiceBytesRecursive {
		switch strings.ToLower(bioEntry.Op) {
		case "read":
			blkRead = blkRead + bioEntry.Value
		case "write":
			blkWrite = blkWrite + bioEntry.Value
		}
	}
	return
}

func calculateNetwork(network map[string]types.NetworkStats) (float64, float64) {
	var rx, tx float64

	for _, v := range network {
		rx += float64(v.RxBytes)
		tx += float64(v.TxBytes)
	}
	return rx, tx
}

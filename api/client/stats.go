package client

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/docker/docker/api/types"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/units"
	"github.com/docker/docker/utils"
)

type containerStats struct {
	Name             string
	CpuPercentage    float64
	Memory           float64
	MemoryLimit      float64
	MemoryPercentage float64
	NetworkRx        float64
	NetworkTx        float64
	mu               sync.RWMutex
	err              error
}

func (s *containerStats) Collect(cli *DockerCli) {
	stream, _, err := cli.call("GET", "/containers/"+s.Name+"/stats", nil, false)
	if err != nil {
		s.err = err
		return
	}
	defer stream.Close()
	var (
		previousCpu    uint64
		previousSystem uint64
		start          = true
		dec            = json.NewDecoder(stream)
		u              = make(chan error, 1)
	)
	go func() {
		for {
			var v *types.Stats
			if err := dec.Decode(&v); err != nil {
				u <- err
				return
			}
			var (
				memPercent = float64(v.MemoryStats.Usage) / float64(v.MemoryStats.Limit) * 100.0
				cpuPercent = 0.0
			)
			if !start {
				cpuPercent = calculateCpuPercent(previousCpu, previousSystem, v)
			}
			start = false
			s.mu.Lock()
			s.CpuPercentage = cpuPercent
			s.Memory = float64(v.MemoryStats.Usage)
			s.MemoryLimit = float64(v.MemoryStats.Limit)
			s.MemoryPercentage = memPercent
			s.NetworkRx = float64(v.Network.RxBytes)
			s.NetworkTx = float64(v.Network.TxBytes)
			s.mu.Unlock()
			previousCpu = v.CpuStats.CpuUsage.TotalUsage
			previousSystem = v.CpuStats.SystemUsage
			u <- nil
		}
	}()
	for {
		select {
		case <-time.After(2 * time.Second):
			// zero out the values if we have not received an update within
			// the specified duration.
			s.mu.Lock()
			s.CpuPercentage = 0
			s.Memory = 0
			s.MemoryPercentage = 0
			s.mu.Unlock()
		case err := <-u:
			if err != nil {
				s.mu.Lock()
				s.err = err
				s.mu.Unlock()
				return
			}
		}
	}
}

func (s *containerStats) Display(w io.Writer) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.err != nil {
		return s.err
	}
	fmt.Fprintf(w, "%s\t%.2f%%\t%s/%s\t%.2f%%\t%s/%s\n",
		s.Name,
		s.CpuPercentage,
		units.BytesSize(s.Memory), units.BytesSize(s.MemoryLimit),
		s.MemoryPercentage,
		units.BytesSize(s.NetworkRx), units.BytesSize(s.NetworkTx))
	return nil
}

// CmdStats displays a live stream of resource usage statistics for one or more containers.
//
// This shows real-time information on CPU usage, memory usage, and network I/O.
//
// Usage: docker stats CONTAINER [CONTAINER...]
func (cli *DockerCli) CmdStats(args ...string) error {
	cmd := cli.Subcmd("stats", "CONTAINER [CONTAINER...]", "Display a live stream of one or more containers' resource usage statistics", true)
	cmd.Require(flag.Min, 1)
	utils.ParseFlags(cmd, args, true)

	names := cmd.Args()
	sort.Strings(names)
	var (
		cStats []*containerStats
		w      = tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)
	)
	printHeader := func() {
		fmt.Fprint(cli.out, "\033[2J")
		fmt.Fprint(cli.out, "\033[H")
		fmt.Fprintln(w, "CONTAINER\tCPU %\tMEM USAGE/LIMIT\tMEM %\tNET I/O")
	}
	for _, n := range names {
		s := &containerStats{Name: n}
		cStats = append(cStats, s)
		go s.Collect(cli)
	}
	// do a quick pause so that any failed connections for containers that do not exist are able to be
	// evicted before we display the initial or default values.
	time.Sleep(500 * time.Millisecond)
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
	for _ = range time.Tick(500 * time.Millisecond) {
		printHeader()
		toRemove := []int{}
		for i, s := range cStats {
			if err := s.Display(w); err != nil {
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
	}
	return nil
}

func calculateCpuPercent(previousCpu, previousSystem uint64, v *types.Stats) float64 {
	var (
		cpuPercent = 0.0
		// calculate the change for the cpu usage of the container in between readings
		cpuDelta = float64(v.CpuStats.CpuUsage.TotalUsage - previousCpu)
		// calculate the change for the entire system between readings
		systemDelta = float64(v.CpuStats.SystemUsage - previousSystem)
	)

	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpuPercent = (cpuDelta / systemDelta) * float64(len(v.CpuStats.CpuUsage.PercpuUsage)) * 100.0
	}
	return cpuPercent
}

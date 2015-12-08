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
	"github.com/docker/docker/pkg/jsonmessage"
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

type stats struct {
	mu sync.Mutex
	cs []*containerStats
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
// Usage: docker stats [OPTIONS] [CONTAINER...]
func (cli *DockerCli) CmdStats(args ...string) error {
	cmd := Cli.Subcmd("stats", []string{"[CONTAINER...]"}, Cli.DockerCommands["stats"].Description, true)
	all := cmd.Bool([]string{"a", "-all"}, false, "Show all containers (default shows just running)")
	noStream := cmd.Bool([]string{"-no-stream"}, false, "Disable streaming stats and only pull the first result")

	cmd.ParseFlags(args, true)

	names := cmd.Args()
	showAll := len(names) == 0

	if showAll {
		v := url.Values{}
		if *all {
			v.Set("all", "1")
		}
		body, _, err := readBody(cli.call("GET", "/containers/json?"+v.Encode(), nil, nil))
		if err != nil {
			return err
		}
		var cs []types.Container
		if err := json.Unmarshal(body, &cs); err != nil {
			return err
		}
		for _, c := range cs {
			names = append(names, c.ID[:12])
		}
	}
	if len(names) == 0 && !showAll {
		return fmt.Errorf("No containers found")
	}
	sort.Strings(names)

	var (
		cStats = stats{}
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
		// no need to lock here since only the main goroutine is running here
		cStats.cs = append(cStats.cs, s)
		go s.Collect(cli, !*noStream)
	}
	closeChan := make(chan error)
	if showAll {
		type watch struct {
			cid   string
			event string
			err   error
		}
		getNewContainers := func(c chan<- watch) {
			res, err := cli.call("GET", "/events", nil, nil)
			if err != nil {
				c <- watch{err: err}
				return
			}
			defer res.body.Close()

			dec := json.NewDecoder(res.body)
			for {
				var j *jsonmessage.JSONMessage
				if err := dec.Decode(&j); err != nil {
					c <- watch{err: err}
					return
				}
				c <- watch{j.ID[:12], j.Status, nil}
			}
		}
		go func(stopChan chan<- error) {
			cChan := make(chan watch)
			go getNewContainers(cChan)
			for {
				c := <-cChan
				if c.err != nil {
					stopChan <- c.err
					return
				}
				switch c.event {
				case "create":
					s := &containerStats{Name: c.cid}
					cStats.mu.Lock()
					cStats.cs = append(cStats.cs, s)
					cStats.mu.Unlock()
					go s.Collect(cli, !*noStream)
				case "stop":
				case "die":
					if !*all {
						var remove int
						// cStats cannot be O(1) with a map cause ranging over it would cause
						// containers in stats to move up and down in the list...:(
						cStats.mu.Lock()
						for i, s := range cStats.cs {
							if s.Name == c.cid {
								remove = i
								break
							}
						}
						cStats.cs = append(cStats.cs[:remove], cStats.cs[remove+1:]...)
						cStats.mu.Unlock()
					}
				}
			}
		}(closeChan)
	} else {
		close(closeChan)
	}
	// do a quick pause so that any failed connections for containers that do not exist are able to be
	// evicted before we display the initial or default values.
	time.Sleep(1500 * time.Millisecond)
	var errs []string
	cStats.mu.Lock()
	for _, c := range cStats.cs {
		c.mu.Lock()
		if c.err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", c.Name, c.err))
		}
		c.mu.Unlock()
	}
	cStats.mu.Unlock()
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, ", "))
	}
	for range time.Tick(500 * time.Millisecond) {
		printHeader()
		toRemove := []int{}
		cStats.mu.Lock()
		for i, s := range cStats.cs {
			if err := s.Display(w); err != nil && !*noStream {
				toRemove = append(toRemove, i)
			}
		}
		for j := len(toRemove) - 1; j >= 0; j-- {
			i := toRemove[j]
			cStats.cs = append(cStats.cs[:i], cStats.cs[i+1:]...)
		}
		if len(cStats.cs) == 0 && !showAll {
			return nil
		}
		cStats.mu.Unlock()
		w.Flush()
		if *noStream {
			break
		}
		select {
		case err, ok := <-closeChan:
			if ok {
				if err != nil {
					// this is suppressing "unexpected EOF" in the cli when the
					// daemon restarts so it shudowns cleanly
					if err == io.ErrUnexpectedEOF {
						return nil
					}
					return err
				}
			}
		default:
			// just skip
		}
	}
	return nil
}

func calculateCPUPercent(previousCPU, previousSystem uint64, v *types.StatsJSON) float64 {
	var (
		cpuPercent = 0.0
		// calculate the change for the cpu usage of the container in between readings
		cpuDelta = float64(v.CPUStats.CPUUsage.TotalUsage) - float64(previousCPU)
		// calculate the change for the entire system between readings
		systemDelta = float64(v.CPUStats.SystemUsage) - float64(previousSystem)
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

package container

import (
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/cli/command/formatter"
	"github.com/docker/docker/client"
	"golang.org/x/net/context"
)

type stats struct {
	ostype string
	mu     sync.Mutex
	cs     []*formatter.ContainerStats
	err    error
}

type containerStatsList []*formatter.ContainerStats

func isEmptyStat(fs *formatter.ContainerStats) bool {
	return fs.MemoryLimit == 0
}

func (sl containerStatsList) Len() int {
	return len(sl)
}

func (sl containerStatsList) Swap(i, j int) {
	sl[i], sl[j] = sl[j], sl[i]
}

func (sl containerStatsList) Less(i, j int) bool {
	// put running containers first
	// non-running container will have error "container not running"
	if isEmptyStat(sl[i]) && !isEmptyStat(sl[j]) {
		return false
	} else if !isEmptyStat(sl[i]) && isEmptyStat(sl[j]) {
		return true
	}

	// if both are running/stopped, compare their ids
	if strings.Compare(sl[i].Container, sl[j].Container) <= 0 {
		return true
	}
	return false
}

// daemonOSType is set once we have at least one stat for a container
// from the daemon. It is used to ensure we print the right header based
// on the daemon platform.
var daemonOSType string

func (s *stats) add(cs *formatter.ContainerStats) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.isKnownContainer(cs.Container); !exists {
		s.cs = append(s.cs, cs)
		return true
	}
	return false
}

func (s *stats) remove(id string) {
	s.mu.Lock()
	if i, exists := s.isKnownContainer(id); exists {
		s.cs = append(s.cs[:i], s.cs[i+1:]...)
	}
	s.mu.Unlock()
}

func (s *stats) isKnownContainer(cid string) (int, bool) {
	for i, c := range s.cs {
		if c.Container == cid {
			return i, true
		}
	}
	return -1, false
}

func (s *stats) collectAll(ctx context.Context, cli client.APIClient, all, streamStats bool, waitFirst *sync.WaitGroup) error {
	logrus.Debugf("collecting stats for all")
	var (
		getFirst bool
		u        = make(chan error, 1)
	)

	defer func() {
		// if error happens and we get nothing of stats, release wait group whatever
		if !getFirst {
			getFirst = true
			waitFirst.Done()
		}
	}()

	filter := filters.NewArgs()
	if !all {
		filter.Add("status", "running")
	}

	options := types.StatsAllOptions{
		Filters: filter,
		Stream:  streamStats,
	}
	response, err := cli.ContainerStatsAll(ctx, options)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	daemonOSType = response.OSType
	dec := json.NewDecoder(response.Body)
	go func() {
		for {
			v := make(map[string]*types.StatsJSON)

			if err := dec.Decode(&v); err != nil {
				dec = json.NewDecoder(io.MultiReader(dec.Buffered(), response.Body))
				u <- err
				if err == io.EOF {
					break
				}
				time.Sleep(100 * time.Millisecond)
				continue
			}

			s.mu.Lock()
			s.cs = nil
			for id, statsJSON := range v {
				fs := formatter.NewContainerStats(id[:12], daemonOSType)
				// a running container will never have zero memory limit
				// so a memory limit of 0 indicates the container isn't running
				calculateContainerStats(fs, statsJSON)
				s.cs = append(s.cs, fs)
			}
			s.mu.Unlock()

			sort.Sort(containerStatsList(s.cs))

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
			s.err = errors.New("timeout waiting for stats")
			s.mu.Unlock()
		case err := <-u:
			s.mu.Lock()
			s.err = err
			s.mu.Unlock()
			if err != nil {
				if err == io.EOF {
					return err
				}
				continue
			}
			// if this is the first stat you get, release WaitGroup
			if !getFirst {
				getFirst = true
				waitFirst.Done()
			}
		}
		if !streamStats {
			return nil
		}
	}
}

func collect(ctx context.Context, s *formatter.ContainerStats, cli client.APIClient, streamStats bool, waitFirst *sync.WaitGroup) {
	logrus.Debugf("collecting stats for %s", s.Container)
	var (
		getFirst bool
		u        = make(chan error, 1)
	)

	defer func() {
		// if error happens and we get nothing of stats, release wait group whatever
		if !getFirst {
			getFirst = true
			waitFirst.Done()
		}
	}()

	response, err := cli.ContainerStats(ctx, s.Container, streamStats)
	if err != nil {
		s.SetError(err)
		return
	}
	defer response.Body.Close()

	dec := json.NewDecoder(response.Body)
	go func() {
		for {
			var v *types.StatsJSON
			var err error

			err = dec.Decode(&v)
			if err != nil {
				dec = json.NewDecoder(io.MultiReader(dec.Buffered(), response.Body))
				u <- err
				if err == io.EOF {
					break
				}
				time.Sleep(100 * time.Millisecond)
				continue
			}

			s.OSType = response.OSType

			// a running container will never have zero memory limit
			// so a memory limit of 0 indicates the container isn't running
			calculateContainerStats(s, v)

			u <- err
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
			s.SetErrorAndReset(errors.New("timeout waiting for stats"))
			// if this is the first stat you get, release WaitGroup
			if !getFirst {
				getFirst = true
				waitFirst.Done()
			}
		case err := <-u:
			s.SetError(err)

			// we get containers stats, but container isn't running
			// so we set the error and release the waitFirst WaitGroup
			if err != nil {
				continue
			}
			// if this is the first stat you get, release WaitGroup
			if !getFirst {
				getFirst = true
				waitFirst.Done()
			}
		}
		if !streamStats {
			return
		}
	}
}

func calculateCPUPercentUnix(previousCPU, previousSystem uint64, v *types.StatsJSON) float64 {
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

func calculateCPUPercentWindows(v *types.StatsJSON) float64 {
	// Max number of 100ns intervals between the previous time read and now
	possIntervals := uint64(v.Read.Sub(v.PreRead).Nanoseconds()) // Start with number of ns intervals
	possIntervals /= 100                                         // Convert to number of 100ns intervals
	possIntervals *= uint64(v.NumProcs)                          // Multiple by the number of processors

	// Intervals used
	intervalsUsed := v.CPUStats.CPUUsage.TotalUsage - v.PreCPUStats.CPUUsage.TotalUsage

	// Percentage avoiding divide-by-zero
	if possIntervals > 0 {
		return float64(intervalsUsed) / float64(possIntervals) * 100.0
	}
	return 0.00
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

func calculateContainerStats(s *formatter.ContainerStats, v *types.StatsJSON) {
	var (
		memPercent                  = 0.0
		cpuPercent                  = 0.0
		blkRead, blkWrite           uint64 // Only used on Linux
		previousCPU, previousSystem uint64
		mem                         = 0.0
		memLimit                    = 0.0
		memPerc                     = 0.0
		pidsStatsCurrent            uint64
	)

	if s.OSType != "windows" {
		// MemoryStats.Limit will never be 0 unless the container is not running and we haven't
		// got any data from cgroup
		if v.MemoryStats.Limit != 0 {
			memPercent = float64(v.MemoryStats.Usage) / float64(v.MemoryStats.Limit) * 100.0
		}
		if v.PreCPUStats != nil {
			previousCPU = v.PreCPUStats.CPUUsage.TotalUsage
			previousSystem = v.PreCPUStats.SystemUsage
		}
		cpuPercent = calculateCPUPercentUnix(previousCPU, previousSystem, v)
		blkRead, blkWrite = calculateBlockIO(v.BlkioStats)
		mem = float64(v.MemoryStats.Usage)
		memLimit = float64(v.MemoryStats.Limit)
		memPerc = memPercent
		pidsStatsCurrent = v.PidsStats.Current
	} else {
		if v.PreCPUStats != nil {
			cpuPercent = calculateCPUPercentWindows(v)
		}
		blkRead = v.StorageStats.ReadSizeBytes
		blkWrite = v.StorageStats.WriteSizeBytes
		mem = float64(v.MemoryStats.PrivateWorkingSet)
	}

	netRx, netTx := calculateNetwork(v.Networks)
	s.SetStatistics(formatter.StatsEntry{
		Name:             v.Name,
		ID:               v.ID,
		CPUPercentage:    cpuPercent,
		Memory:           mem,
		MemoryPercentage: memPerc,
		MemoryLimit:      memLimit,
		NetworkRx:        netRx,
		NetworkTx:        netTx,
		BlockRead:        float64(blkRead),
		BlockWrite:       float64(blkWrite),
		PidsCurrent:      pidsStatsCurrent,
	})
}

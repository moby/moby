package client

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/docker/go-units"
	"golang.org/x/net/context"
)

type containerStats struct {
	Name             string
	CPUPercentage    float64
	Memory           float64
	MemoryLimit      float64
	MemoryPercentage float64
	NetworkRx        float64
	NetworkTx        float64
	BlockReadByte    float64
	BlockWriteByte   float64
	BlockReadRate    float64
	BlockWriteRate   float64
	BlockReadIOPS    float64
	BlockWriteIOPS   float64
	PidsCurrent      uint64
	mu               sync.RWMutex
	err              error
}

type stats struct {
	mu sync.Mutex
	cs []*containerStats
}

func (s *stats) add(cs *containerStats) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.isKnownContainer(cs.Name); !exists {
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
		if c.Name == cid {
			return i, true
		}
	}
	return -1, false
}

func (s *containerStats) Collect(cli client.APIClient, streamStats bool, waitFirst *sync.WaitGroup) {
	var (
		getFirst       bool
		previousCPU    uint64
		previousSystem uint64
		u              = make(chan error, 1)
	)

	defer func() {
		// if error happens and we get nothing of stats, release wait group whatever
		if !getFirst {
			getFirst = true
			waitFirst.Done()
		}
	}()

	responseBody, err := cli.ContainerStats(context.Background(), s.Name, streamStats)
	if err != nil {
		s.mu.Lock()
		s.err = err
		s.mu.Unlock()
		return
	}
	defer responseBody.Close()

	dec := json.NewDecoder(responseBody)
	go func() {
		for {
			var v *types.StatsJSON
			if err := dec.Decode(&v); err != nil {
				u <- err
				return
			}

			var memPercent = 0.0
			var cpuPercent = 0.0

			// MemoryStats.Limit will never be 0 unless the container is not running and we haven't
			// got any data from cgroup
			if v.MemoryStats.Limit != 0 {
				memPercent = float64(v.MemoryStats.Usage) / float64(v.MemoryStats.Limit) * 100.0
			}

			previousCPU = v.PreCPUStats.CPUUsage.TotalUsage
			previousSystem = v.PreCPUStats.SystemUsage
			cpuPercent = calculateCPUPercent(previousCPU, previousSystem, v)
			blkReadByte, blkWriteByte, blkReadRate, blkWriteRate, blkReadIOPS, blkWriteIOPS := calculateBlockIO(v.BlkioStats, v.PreBlkioStats, v.Read, v.PreRead)
			s.mu.Lock()
			s.CPUPercentage = cpuPercent
			s.Memory = float64(v.MemoryStats.Usage)
			s.MemoryLimit = float64(v.MemoryStats.Limit)
			s.MemoryPercentage = memPercent
			s.NetworkRx, s.NetworkTx = calculateNetwork(v.Networks)
			s.BlockReadByte = float64(blkReadByte)
			s.BlockWriteByte = float64(blkWriteByte)
			s.BlockReadRate = float64(blkReadRate)
			s.BlockWriteRate = float64(blkWriteRate)
			s.BlockReadIOPS = blkReadIOPS
			s.BlockWriteIOPS = blkWriteIOPS
			s.PidsCurrent = v.PidsStats.Current
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
			s.BlockReadByte = 0
			s.BlockWriteByte = 0
			s.BlockReadRate = 0
			s.BlockWriteRate = 0
			s.BlockReadIOPS = 0
			s.BlockWriteIOPS = 0
			s.PidsCurrent = 0
			s.mu.Unlock()
			// if this is the first stat you get, release WaitGroup
			if !getFirst {
				getFirst = true
				waitFirst.Done()
			}
		case err := <-u:
			if err != nil {
				s.mu.Lock()
				s.err = err
				s.mu.Unlock()
				return
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

func (s *containerStats) Display(w io.Writer) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.err != nil {
		return s.err
	}
	fmt.Fprintf(w, "%s\t%.2f%%\t%s / %s\t%.2f%%\t%s / %s\t%s / %s\t%s / %s\t%.2f / %.2f\t%d\n",
		s.Name,
		s.CPUPercentage,
		units.HumanSize(s.Memory), units.HumanSize(s.MemoryLimit),
		s.MemoryPercentage,
		units.HumanSize(s.NetworkRx), units.HumanSize(s.NetworkTx),
		units.HumanSize(s.BlockReadByte), units.HumanSize(s.BlockWriteByte),
		units.HumanSize(s.BlockReadRate), units.HumanSize(s.BlockWriteRate),
		s.BlockReadIOPS, s.BlockWriteIOPS, s.PidsCurrent)
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

func calculateBlockIO(blkio, preblkio types.BlkioStats, read, preread time.Time) (blkReadByte uint64, blkWriteByte uint64,
	blkReadRate uint64, blkWriteRate uint64,
	blkReadIOPS float64, blkWriteIOPS float64) {
	var (
		preblkReadByte   uint64
		preblkWriteByte  uint64
		blkReadCount     uint64
		blkWriteCount    uint64
		preblkReadCount  uint64
		preblkWriteCount uint64
		blkReadDelta     uint64
		blkWriteDelta    uint64
		duration         uint64
	)

	for _, bioEntry := range blkio.IoServiceBytesRecursive {
		switch strings.ToLower(bioEntry.Op) {
		case "read":
			blkReadByte = blkReadByte + bioEntry.Value
		case "write":
			blkWriteByte = blkWriteByte + bioEntry.Value
		}
	}

	for _, bioEntry := range preblkio.IoServiceBytesRecursive {
		switch strings.ToLower(bioEntry.Op) {
		case "read":
			preblkReadByte = preblkReadByte + bioEntry.Value
		case "write":
			preblkWriteByte = preblkWriteByte + bioEntry.Value
		}
	}

	for _, bioEntry := range blkio.IoServicedRecursive {
		switch strings.ToLower(bioEntry.Op) {
		case "read":
			blkReadCount = blkReadCount + bioEntry.Value
		case "write":
			blkWriteCount = blkWriteCount + bioEntry.Value
		}
	}

	for _, bioEntry := range preblkio.IoServicedRecursive {
		switch strings.ToLower(bioEntry.Op) {
		case "read":
			preblkReadCount = preblkReadCount + bioEntry.Value
		case "write":
			preblkWriteCount = preblkWriteCount + bioEntry.Value
		}
	}

	if read.After(preread) {
		duration = uint64(read.Sub(preread))
		// convert it to Millisecond
		duration = duration / uint64(time.Millisecond)
		// calculate the Rate in 1s
		blkReadDelta = blkReadByte - preblkReadByte
		blkWriteDelta = blkWriteByte - preblkWriteByte
		blkReadRate = blkReadDelta * 1000 / duration
		blkWriteRate = blkWriteDelta * 1000 / duration
		// calculate the IOPS
		blkReadDelta = blkReadCount - preblkReadCount
		blkWriteDelta = blkWriteCount - preblkWriteCount
		blkReadIOPS = float64(blkReadDelta) * 1000.0 / float64(duration)
		blkWriteIOPS = float64(blkWriteDelta) * 1000.0 / float64(duration)
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

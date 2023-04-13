package daemon // import "github.com/docker/docker/daemon"

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/container"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/platform"
)

func (daemon *Daemon) stats(c *container.Container) (*types.StatsJSON, error) {
	c.Lock()
	task, err := c.GetRunningTask()
	c.Unlock()
	if err != nil {
		return nil, err
	}

	// Obtain the stats from HCS via libcontainerd
	stats, err := task.Stats(context.Background())
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil, containerNotFound(c.ID)
		}
		return nil, err
	}

	// Start with an empty structure
	s := &types.StatsJSON{}
	s.Stats.Read = stats.Read
	s.Stats.NumProcs = platform.NumProcs()

	if stats.HCSStats != nil {
		hcss := stats.HCSStats
		// Populate the CPU/processor statistics
		s.CPUStats = types.CPUStats{
			CPUUsage: types.CPUUsage{
				TotalUsage:        hcss.Processor.TotalRuntime100ns,
				UsageInKernelmode: hcss.Processor.RuntimeKernel100ns,
				UsageInUsermode:   hcss.Processor.RuntimeUser100ns,
			},
		}

		// Populate the memory statistics
		s.MemoryStats = types.MemoryStats{
			Commit:            hcss.Memory.UsageCommitBytes,
			CommitPeak:        hcss.Memory.UsageCommitPeakBytes,
			PrivateWorkingSet: hcss.Memory.UsagePrivateWorkingSetBytes,
		}

		// Populate the storage statistics
		s.StorageStats = types.StorageStats{
			ReadCountNormalized:  hcss.Storage.ReadCountNormalized,
			ReadSizeBytes:        hcss.Storage.ReadSizeBytes,
			WriteCountNormalized: hcss.Storage.WriteCountNormalized,
			WriteSizeBytes:       hcss.Storage.WriteSizeBytes,
		}

		// Populate the network statistics
		s.Networks = make(map[string]types.NetworkStats)
		for _, nstats := range hcss.Network {
			s.Networks[nstats.EndpointId] = types.NetworkStats{
				RxBytes:   nstats.BytesReceived,
				RxPackets: nstats.PacketsReceived,
				RxDropped: nstats.DroppedPacketsIncoming,
				TxBytes:   nstats.BytesSent,
				TxPackets: nstats.PacketsSent,
				TxDropped: nstats.DroppedPacketsOutgoing,
			}
		}
	}
	return s, nil
}

// Windows network stats are obtained directly through HCS, hence this is a no-op.
func (daemon *Daemon) getNetworkStats(c *container.Container) (map[string]types.NetworkStats, error) {
	return make(map[string]types.NetworkStats), nil
}

package daemon

import (
	"context"
	"runtime"

	"github.com/Microsoft/hcsshim"
	cerrdefs "github.com/containerd/errdefs"
	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/daemon/container"
	"github.com/moby/moby/v2/daemon/internal/platform"
)

func (daemon *Daemon) stats(c *container.Container) (*containertypes.StatsResponse, error) {
	c.Lock()
	task, err := c.GetRunningTask()
	c.Unlock()
	if err != nil {
		return nil, err
	}

	// Obtain the stats from HCS via libcontainerd
	stats, err := task.Stats(context.Background())
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			return nil, containerNotFound(c.ID)
		}
		return nil, err
	}

	// Start with an empty structure
	s := &containertypes.StatsResponse{
		ID:       c.ID,
		Name:     c.Name,
		OSType:   runtime.GOOS,
		Read:     stats.Read,
		NumProcs: platform.NumProcs(),
	}

	if stats.HCSStats != nil {
		hcss := stats.HCSStats
		// Populate the CPU/processor statistics
		s.CPUStats = containertypes.CPUStats{
			CPUUsage: containertypes.CPUUsage{
				TotalUsage:        hcss.Processor.TotalRuntime100ns,
				UsageInKernelmode: hcss.Processor.RuntimeKernel100ns,
				UsageInUsermode:   hcss.Processor.RuntimeUser100ns,
			},
		}

		// Populate the memory statistics
		s.MemoryStats = containertypes.MemoryStats{
			Commit:            hcss.Memory.UsageCommitBytes,
			CommitPeak:        hcss.Memory.UsageCommitPeakBytes,
			PrivateWorkingSet: hcss.Memory.UsagePrivateWorkingSetBytes,
		}

		// Populate the storage statistics
		s.StorageStats = containertypes.StorageStats{
			ReadCountNormalized:  hcss.Storage.ReadCountNormalized,
			ReadSizeBytes:        hcss.Storage.ReadSizeBytes,
			WriteCountNormalized: hcss.Storage.WriteCountNormalized,
			WriteSizeBytes:       hcss.Storage.WriteSizeBytes,
		}

		// Populate the network statistics
		s.Networks = make(map[string]containertypes.NetworkStats)
		for _, nstats := range hcss.Network {
			s.Networks[nstats.EndpointId] = containertypes.NetworkStats{
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

// getNetworkStats collects network statistics for a container. The builtin HCS
// runtime already embeds these in the container stats, but the containerd
// runtime does not, so they are queried here directly from HNS by endpoint.
func (daemon *Daemon) getNetworkStats(c *container.Container) (map[string]containertypes.NetworkStats, error) {
	sandboxID, err := daemon.getNetworkSandboxID(c)
	if err != nil {
		return nil, err
	}

	sb, err := daemon.netController.SandboxByID(sandboxID)
	if err != nil {
		return nil, err
	}

	stats := make(map[string]containertypes.NetworkStats)
	for _, ep := range sb.Endpoints() {
		info, err := ep.DriverInfo()
		if err != nil {
			return nil, err
		}
		hnsid, ok := info["hnsid"].(string)
		if !ok || hnsid == "" {
			continue
		}

		epStats, err := hcsshim.GetHNSEndpointStats(hnsid)
		if err != nil {
			return nil, err
		}

		stats[epStats.EndpointID] = hnsStatsToNetworkStats(epStats)
	}
	return stats, nil
}

// hnsStatsToNetworkStats maps HNS endpoint statistics onto the API network stats
// shape. Errors are not reported by HNS, so RxErrors/TxErrors are left zeroed
// (matching the builtin HCS path, which also does not populate them on Windows).
func hnsStatsToNetworkStats(epStats *hcsshim.HNSEndpointStats) containertypes.NetworkStats {
	return containertypes.NetworkStats{
		RxBytes:    epStats.BytesReceived,
		RxPackets:  epStats.PacketsReceived,
		RxDropped:  epStats.DroppedPacketsIncoming,
		TxBytes:    epStats.BytesSent,
		TxPackets:  epStats.PacketsSent,
		TxDropped:  epStats.DroppedPacketsOutgoing,
		EndpointID: epStats.EndpointID,
		InstanceID: epStats.InstanceID,
	}
}

// getSystemCPUUsage returns the host system's cpu usage in
// nanoseconds and number of online CPUs. An error is returned
// if the format of the underlying file does not match.
// This is a no-op on Windows.
func getSystemCPUUsage() (uint64, uint32, error) {
	return 0, 0, nil
}

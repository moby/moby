package daemon

import (
	"github.com/docker/docker/api/types"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/cgroups"
)

// convertStatsToAPITypes converts the libcontainer.Stats to the api specific
// structs. This is done to preserve API compatibility and versioning.
func convertStatsToAPITypes(ls *libcontainer.Stats) *types.StatsJSON {
	s := &types.StatsJSON{}
	if ls.Interfaces != nil {
		s.Networks = make(map[string]types.NetworkStats)
		for _, iface := range ls.Interfaces {
			// For API Version >= 1.21, the original data of network will
			// be returned.
			s.Networks[iface.Name] = types.NetworkStats{
				RxBytes:   iface.RxBytes,
				RxPackets: iface.RxPackets,
				RxErrors:  iface.RxErrors,
				RxDropped: iface.RxDropped,
				TxBytes:   iface.TxBytes,
				TxPackets: iface.TxPackets,
				TxErrors:  iface.TxErrors,
				TxDropped: iface.TxDropped,
			}
		}
	}

	cs := ls.CgroupStats
	if cs != nil {
		s.BlkioStats = types.BlkioStats{
			IoServiceBytesRecursive: copyBlkioEntry(cs.BlkioStats.IoServiceBytesRecursive),
			IoServicedRecursive:     copyBlkioEntry(cs.BlkioStats.IoServicedRecursive),
			IoQueuedRecursive:       copyBlkioEntry(cs.BlkioStats.IoQueuedRecursive),
			IoServiceTimeRecursive:  copyBlkioEntry(cs.BlkioStats.IoServiceTimeRecursive),
			IoWaitTimeRecursive:     copyBlkioEntry(cs.BlkioStats.IoWaitTimeRecursive),
			IoMergedRecursive:       copyBlkioEntry(cs.BlkioStats.IoMergedRecursive),
			IoTimeRecursive:         copyBlkioEntry(cs.BlkioStats.IoTimeRecursive),
			SectorsRecursive:        copyBlkioEntry(cs.BlkioStats.SectorsRecursive),
		}
		cpu := cs.CpuStats
		s.CPUStats = types.CPUStats{
			CPUUsage: types.CPUUsage{
				TotalUsage:        cpu.CpuUsage.TotalUsage,
				PercpuUsage:       cpu.CpuUsage.PercpuUsage,
				UsageInKernelmode: cpu.CpuUsage.UsageInKernelmode,
				UsageInUsermode:   cpu.CpuUsage.UsageInUsermode,
			},
			ThrottlingData: types.ThrottlingData{
				Periods:          cpu.ThrottlingData.Periods,
				ThrottledPeriods: cpu.ThrottlingData.ThrottledPeriods,
				ThrottledTime:    cpu.ThrottlingData.ThrottledTime,
			},
		}
		mem := cs.MemoryStats
		s.MemoryStats = types.MemoryStats{
			Usage:    mem.Usage.Usage,
			MaxUsage: mem.Usage.MaxUsage,
			Stats:    mem.Stats,
			Failcnt:  mem.Usage.Failcnt,
		}
	}

	return s
}

func copyBlkioEntry(entries []cgroups.BlkioStatEntry) []types.BlkioStatEntry {
	out := make([]types.BlkioStatEntry, len(entries))
	for i, re := range entries {
		out[i] = types.BlkioStatEntry{
			Major: re.Major,
			Minor: re.Minor,
			Op:    re.Op,
			Value: re.Value,
		}
	}
	return out
}

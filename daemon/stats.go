package daemon

import (
	"encoding/json"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/libcontainer"
	"github.com/docker/libcontainer/cgroups"
)

func (daemon *Daemon) ContainerStats(name string, stream bool, out io.Writer) error {
	updates, err := daemon.SubscribeToContainerStats(name)
	if err != nil {
		return err
	}

	var preCpuStats types.CpuStats
	getStat := func(v interface{}) *types.Stats {
		update := v.(*execdriver.ResourceStats)
		ss := convertToAPITypes(update.Stats)
		ss.PreCpuStats = preCpuStats
		ss.MemoryStats.Limit = uint64(update.MemoryLimit)
		ss.Read = update.Read
		ss.CpuStats.SystemUsage = update.SystemUsage
		preCpuStats = ss.CpuStats
		return ss
	}

	enc := json.NewEncoder(out)

	if !stream {
		// prime the cpu stats so they aren't 0 in the final output
		s := getStat(<-updates)

		// now pull stats again with the cpu stats primed
		s = getStat(<-updates)
		err := enc.Encode(s)
		daemon.UnsubscribeToContainerStats(name, updates)
		return err
	}

	for v := range updates {
		s := getStat(v)
		if err := enc.Encode(s); err != nil {
			// TODO: handle the specific broken pipe
			daemon.UnsubscribeToContainerStats(name, updates)
			return err
		}
	}
	return nil
}

// convertToAPITypes converts the libcontainer.Stats to the api specific
// structs.  This is done to preserve API compatibility and versioning.
func convertToAPITypes(ls *libcontainer.Stats) *types.Stats {
	s := &types.Stats{}
	if ls.Interfaces != nil {
		s.Network = types.Network{}
		for _, iface := range ls.Interfaces {
			s.Network.RxBytes += iface.RxBytes
			s.Network.RxPackets += iface.RxPackets
			s.Network.RxErrors += iface.RxErrors
			s.Network.RxDropped += iface.RxDropped
			s.Network.TxBytes += iface.TxBytes
			s.Network.TxPackets += iface.TxPackets
			s.Network.TxErrors += iface.TxErrors
			s.Network.TxDropped += iface.TxDropped
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
		s.CpuStats = types.CpuStats{
			CpuUsage: types.CpuUsage{
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
			Usage:    mem.Usage,
			MaxUsage: mem.MaxUsage,
			Stats:    mem.Stats,
			Failcnt:  mem.Failcnt,
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

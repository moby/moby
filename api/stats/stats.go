package stats

import (
	"time"

	"github.com/docker/libcontainer"
	"github.com/docker/libcontainer/cgroups"
)

type ThrottlingData struct {
	// Number of periods with throttling active
	Periods uint64 `json:"periods,omitempty"`
	// Number of periods when the container hit its throttling limit.
	ThrottledPeriods uint64 `json:"throttled_periods,omitempty"`
	// Aggregate time the container was throttled for in nanoseconds.
	ThrottledTime uint64 `json:"throttled_time,omitempty"`
}

// All CPU stats are aggregated since container inception.
type CpuUsage struct {
	// Total CPU time consumed.
	// Units: nanoseconds.
	TotalUsage uint64 `json:"total_usage,omitempty"`
	// Total CPU time consumed per core.
	// Units: nanoseconds.
	PercpuUsage []uint64 `json:"percpu_usage,omitempty"`
	// Time spent by tasks of the cgroup in kernel mode.
	// Units: nanoseconds.
	UsageInKernelmode uint64 `json:"usage_in_kernelmode"`
	// Time spent by tasks of the cgroup in user mode.
	// Units: nanoseconds.
	UsageInUsermode uint64 `json:"usage_in_usermode"`
}

type CpuStats struct {
	CpuUsage       CpuUsage       `json:"cpu_usage,omitempty"`
	SystemUsage    uint64         `json:"system_cpu_usage"`
	ThrottlingData ThrottlingData `json:"throttling_data,omitempty"`
}

type MemoryStats struct {
	// current res_counter usage for memory
	Usage uint64 `json:"usage,omitempty"`
	// maximum usage ever recorded.
	MaxUsage uint64 `json:"max_usage,omitempty"`
	// TODO(vishh): Export these as stronger types.
	// all the stats exported via memory.stat.
	Stats map[string]uint64 `json:"stats,omitempty"`
	// number of times memory usage hits limits.
	Failcnt uint64 `json:"failcnt"`
	Limit   uint64 `json:"limit"`
}

type BlkioStatEntry struct {
	Major uint64 `json:"major,omitempty"`
	Minor uint64 `json:"minor,omitempty"`
	Op    string `json:"op,omitempty"`
	Value uint64 `json:"value,omitempty"`
}

type BlkioStats struct {
	// number of bytes tranferred to and from the block device
	IoServiceBytesRecursive []BlkioStatEntry `json:"io_service_bytes_recursive,omitempty"`
	IoServicedRecursive     []BlkioStatEntry `json:"io_serviced_recursive,omitempty"`
	IoQueuedRecursive       []BlkioStatEntry `json:"io_queue_recursive,omitempty"`
	IoServiceTimeRecursive  []BlkioStatEntry `json:"io_service_time_recursive,omitempty"`
	IoWaitTimeRecursive     []BlkioStatEntry `json:"io_wait_time_recursive,omitempty"`
	IoMergedRecursive       []BlkioStatEntry `json:"io_merged_recursive,omitempty"`
	IoTimeRecursive         []BlkioStatEntry `json:"io_time_recursive,omitempty"`
	SectorsRecursive        []BlkioStatEntry `json:"sectors_recursive,omitempty"`
}

type Network struct {
	RxBytes   uint64 `json:"rx_bytes"`
	RxPackets uint64 `json:"rx_packets"`
	RxErrors  uint64 `json:"rx_errors"`
	RxDropped uint64 `json:"rx_dropped"`
	TxBytes   uint64 `json:"tx_bytes"`
	TxPackets uint64 `json:"tx_packets"`
	TxErrors  uint64 `json:"tx_errors"`
	TxDropped uint64 `json:"tx_dropped"`
}

type Stats struct {
	Read        time.Time   `json:"read"`
	Network     Network     `json:"network,omitempty"`
	CpuStats    CpuStats    `json:"cpu_stats,omitempty"`
	MemoryStats MemoryStats `json:"memory_stats,omitempty"`
	BlkioStats  BlkioStats  `json:"blkio_stats,omitempty"`
}

// ToStats converts the libcontainer.ContainerStats to the api specific
// structs.  This is done to preserve API compatibility and versioning.
func ToStats(ls *libcontainer.ContainerStats) *Stats {
	s := &Stats{}
	if ls.NetworkStats != nil {
		s.Network = Network{
			RxBytes:   ls.NetworkStats.RxBytes,
			RxPackets: ls.NetworkStats.RxPackets,
			RxErrors:  ls.NetworkStats.RxErrors,
			RxDropped: ls.NetworkStats.RxDropped,
			TxBytes:   ls.NetworkStats.TxBytes,
			TxPackets: ls.NetworkStats.TxPackets,
			TxErrors:  ls.NetworkStats.TxErrors,
			TxDropped: ls.NetworkStats.TxDropped,
		}
	}
	cs := ls.CgroupStats
	if cs != nil {
		s.BlkioStats = BlkioStats{
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
		s.CpuStats = CpuStats{
			CpuUsage: CpuUsage{
				TotalUsage:        cpu.CpuUsage.TotalUsage,
				PercpuUsage:       cpu.CpuUsage.PercpuUsage,
				UsageInKernelmode: cpu.CpuUsage.UsageInKernelmode,
				UsageInUsermode:   cpu.CpuUsage.UsageInUsermode,
			},
			ThrottlingData: ThrottlingData{
				Periods:          cpu.ThrottlingData.Periods,
				ThrottledPeriods: cpu.ThrottlingData.ThrottledPeriods,
				ThrottledTime:    cpu.ThrottlingData.ThrottledTime,
			},
		}
		mem := cs.MemoryStats
		s.MemoryStats = MemoryStats{
			Usage:    mem.Usage,
			MaxUsage: mem.MaxUsage,
			Stats:    mem.Stats,
			Failcnt:  mem.Failcnt,
		}
	}
	return s
}

func copyBlkioEntry(entries []cgroups.BlkioStatEntry) []BlkioStatEntry {
	out := make([]BlkioStatEntry, len(entries))
	for i, re := range entries {
		out[i] = BlkioStatEntry{
			Major: re.Major,
			Minor: re.Minor,
			Op:    re.Op,
			Value: re.Value,
		}
	}
	return out
}

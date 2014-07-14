package cgroups

type ThrottlingData struct {
	// Number of periods with throttling active
	Periods uint64 `json:"periods,omitempty"`
	// Number of periods when the container hit its throttling limit.
	ThrottledPeriods uint64 `json:"throttled_periods,omitempty"`
	// Aggregate time the container was throttled for in nanoseconds.
	ThrottledTime uint64 `json:"throttled_time,omitempty"`
}

type CpuUsage struct {
	// percentage of available CPUs currently being used.
	PercentUsage uint64 `json:"percent_usage,omitempty"`
	// nanoseconds of cpu time consumed over the last 100 ms.
	CurrentUsage uint64 `json:"current_usage,omitempty"`
	// total nanoseconds of cpu time consumed
	TotalUsage  uint64   `json:"total_usage,omitempty"`
	PercpuUsage []uint64 `json:"percpu_usage,omitempty"`
	// Time spent by tasks of the cgroup in kernel mode. Units: nanoseconds.
	UsageInKernelmode uint64 `json:"usage_in_kernelmode"`
	// Time spent by tasks of the cgroup in user mode. Units: nanoseconds.
	UsageInUsermode uint64 `json:"usage_in_usermode"`
}

type CpuStats struct {
	CpuUsage       CpuUsage       `json:"cpu_usage,omitempty"`
	ThrottlingData ThrottlingData `json:"throlling_data,omitempty"`
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
	IoServicedRecursive     []BlkioStatEntry `json:"io_serviced_recusrive,omitempty"`
	IoQueuedRecursive       []BlkioStatEntry `json:"io_queue_recursive,omitempty"`
	SectorsRecursive        []BlkioStatEntry `json:"sectors_recursive,omitempty"`
}

type Stats struct {
	CpuStats    CpuStats    `json:"cpu_stats,omitempty"`
	MemoryStats MemoryStats `json:"memory_stats,omitempty"`
	BlkioStats  BlkioStats  `json:"blkio_stats,omitempty"`
}

func NewStats() *Stats {
	memoryStats := MemoryStats{Stats: make(map[string]uint64)}
	return &Stats{MemoryStats: memoryStats}
}

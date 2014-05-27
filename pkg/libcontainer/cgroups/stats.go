package cgroups

type ThrottlingData struct {
	// Number of periods with throttling active
	Periods int64 `json:"periods,omitempty"`
	// Number of periods when the container hit its throttling limit.
	ThrottledPeriods int64 `json:"throttled_periods,omitempty"`
	// Aggregate time the container was throttled for in nanoseconds.
	ThrottledTime int64 `json:"throttled_time,omitempty"`
}

type CpuUsage struct {
	// percentage of available CPUs currently being used.
	PercentUsage int64 `json:"percent_usage,omitempty"`
	// nanoseconds of cpu time consumed over the last 100 ms.
	CurrentUsage int64 `json:"current_usage,omitempty"`
}

type CpuStats struct {
	CpuUsage       CpuUsage       `json:"cpu_usage,omitempty"`
	ThrottlingData ThrottlingData `json:"throlling_data,omitempty"`
}

type MemoryStats struct {
	// current res_counter usage for memory
	Usage int64 `json:"usage,omitempty"`
	// maximum usage ever recorded.
	MaxUsage int64 `json:"max_usage,omitempty"`
	// TODO(vishh): Export these as stronger types.
	// all the stats exported via memory.stat.
	Stats map[string]int64 `json:"stats,omitempty"`
}

type BlkioStatEntry struct {
	Major int64  `json:"major,omitempty"`
	Minor int64  `json:"minor,omitempty"`
	Op    string `json:"op,omitempty"`
	Value int64  `json:"value,omitempty"`
}

type BlockioStats struct {
	// number of bytes tranferred to and from the block device
	IoServiceBytesRecursive []BlkioStatEntry `json:"io_service_bytes_recursive,omitempty"`
	IoServicedRecursive     []BlkioStatEntry `json:"io_serviced_recusrive,omitempty"`
	IoQueuedRecursive       []BlkioStatEntry `json:"io_queue_recursive,omitempty"`
}

// TODO(Vishh): Remove freezer from stats since it does not logically belong in stats.
type FreezerStats struct {
	ParentState string `json:"parent_state,omitempty"`
	SelfState   string `json:"self_state,omitempty"`
}

type Stats struct {
	CpuStats     CpuStats     `json:"cpu_stats,omitempty"`
	MemoryStats  MemoryStats  `json:"memory_stats,omitempty"`
	BlockioStats BlockioStats `json:"blockio_stats,omitempty"`
	FreezerStats FreezerStats `json:"freezer_stats,omitempty"`
}

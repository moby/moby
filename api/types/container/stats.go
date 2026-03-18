package container

import "time"

// ThrottlingData stores CPU throttling stats of one running container.
// Not used on Windows.
type ThrottlingData struct {
	// Number of periods with throttling active
	Periods uint64 `json:"periods"`
	// Number of periods when the container hits its throttling limit.
	ThrottledPeriods uint64 `json:"throttled_periods"`
	// Aggregate time the container was throttled for in nanoseconds.
	ThrottledTime uint64 `json:"throttled_time"`
}

// CPUUsage stores All CPU stats aggregated since container inception.
type CPUUsage struct {
	// Total CPU time consumed.
	// Units: nanoseconds (Linux)
	// Units: 100's of nanoseconds (Windows)
	TotalUsage uint64 `json:"total_usage"`

	// Total CPU time consumed per core (Linux). Not used on Windows.
	// Units: nanoseconds.
	PercpuUsage []uint64 `json:"percpu_usage,omitempty"`

	// Time spent by tasks of the cgroup in kernel mode (Linux).
	// Time spent by all container processes in kernel mode (Windows).
	// Units: nanoseconds (Linux).
	// Units: 100's of nanoseconds (Windows). Not populated for Hyper-V Containers.
	UsageInKernelmode uint64 `json:"usage_in_kernelmode"`

	// Time spent by tasks of the cgroup in user mode (Linux).
	// Time spent by all container processes in user mode (Windows).
	// Units: nanoseconds (Linux).
	// Units: 100's of nanoseconds (Windows). Not populated for Hyper-V Containers
	UsageInUsermode uint64 `json:"usage_in_usermode"`
}

// CPUStats aggregates and wraps all CPU related info of container
type CPUStats struct {
	// CPU Usage. Linux and Windows.
	CPUUsage CPUUsage `json:"cpu_usage"`

	// System Usage. Linux only.
	SystemUsage uint64 `json:"system_cpu_usage,omitempty"`

	// Online CPUs. Linux only.
	OnlineCPUs uint32 `json:"online_cpus,omitempty"`

	// Throttling Data. Linux only.
	ThrottlingData ThrottlingData `json:"throttling_data,omitempty"`
}

// MemoryStats aggregates all memory stats since container inception on Linux.
// Windows returns stats for commit and private working set only.
type MemoryStats struct {
	// Linux Memory Stats

	// current res_counter usage for memory
	Usage uint64 `json:"usage,omitempty"`
	// maximum usage ever recorded.
	MaxUsage uint64 `json:"max_usage,omitempty"`
	// TODO(vishh): Export these as stronger types.
	// all the stats exported via memory.stat.
	Stats map[string]uint64 `json:"stats,omitempty"`
	// number of times memory usage hits limits.
	Failcnt uint64 `json:"failcnt,omitempty"`
	Limit   uint64 `json:"limit,omitempty"`

	// Windows Memory Stats
	// See https://technet.microsoft.com/en-us/magazine/ff382715.aspx

	// committed bytes
	Commit uint64 `json:"commitbytes,omitempty"`
	// peak committed bytes
	CommitPeak uint64 `json:"commitpeakbytes,omitempty"`
	// private working set
	PrivateWorkingSet uint64 `json:"privateworkingset,omitempty"`
}

// BlkioStatEntry is one small entity to store a piece of Blkio stats
// Not used on Windows.
type BlkioStatEntry struct {
	Major uint64 `json:"major"`
	Minor uint64 `json:"minor"`
	Op    string `json:"op"`
	Value uint64 `json:"value"`
}

// BlkioStats stores All IO service stats for data read and write.
// This is a Linux specific structure as the differences between expressing
// block I/O on Windows and Linux are sufficiently significant to make
// little sense attempting to morph into a combined structure.
type BlkioStats struct {
	// number of bytes transferred to and from the block device
	IoServiceBytesRecursive []BlkioStatEntry `json:"io_service_bytes_recursive"`
	IoServicedRecursive     []BlkioStatEntry `json:"io_serviced_recursive"`
	IoQueuedRecursive       []BlkioStatEntry `json:"io_queue_recursive"`
	IoServiceTimeRecursive  []BlkioStatEntry `json:"io_service_time_recursive"`
	IoWaitTimeRecursive     []BlkioStatEntry `json:"io_wait_time_recursive"`
	IoMergedRecursive       []BlkioStatEntry `json:"io_merged_recursive"`
	IoTimeRecursive         []BlkioStatEntry `json:"io_time_recursive"`
	SectorsRecursive        []BlkioStatEntry `json:"sectors_recursive"`
}

// StorageStats is the disk I/O stats for read/write on Windows.
type StorageStats struct {
	ReadCountNormalized  uint64 `json:"read_count_normalized,omitempty"`
	ReadSizeBytes        uint64 `json:"read_size_bytes,omitempty"`
	WriteCountNormalized uint64 `json:"write_count_normalized,omitempty"`
	WriteSizeBytes       uint64 `json:"write_size_bytes,omitempty"`
}

// NetworkStats aggregates the network stats of one container
type NetworkStats struct {
	// Bytes received. Windows and Linux.
	RxBytes uint64 `json:"rx_bytes"`
	// Packets received. Windows and Linux.
	RxPackets uint64 `json:"rx_packets"`
	// Received errors. Not used on Windows. Note that we don't `omitempty` this
	// field as it is expected in the >=v1.21 API stats structure.
	RxErrors uint64 `json:"rx_errors"`
	// Incoming packets dropped. Windows and Linux.
	RxDropped uint64 `json:"rx_dropped"`
	// Bytes sent. Windows and Linux.
	TxBytes uint64 `json:"tx_bytes"`
	// Packets sent. Windows and Linux.
	TxPackets uint64 `json:"tx_packets"`
	// Sent errors. Not used on Windows. Note that we don't `omitempty` this
	// field as it is expected in the >=v1.21 API stats structure.
	TxErrors uint64 `json:"tx_errors"`
	// Outgoing packets dropped. Windows and Linux.
	TxDropped uint64 `json:"tx_dropped"`
	// Endpoint ID. Not used on Linux.
	EndpointID string `json:"endpoint_id,omitempty"`
	// Instance ID. Not used on Linux.
	InstanceID string `json:"instance_id,omitempty"`
}

// PidsStats contains the stats of a container's pids
type PidsStats struct {
	// Current is the number of pids in the cgroup
	Current uint64 `json:"current,omitempty"`
	// Limit is the hard limit on the number of pids in the cgroup.
	// A "Limit" of 0 means that there is no limit.
	Limit uint64 `json:"limit,omitempty"`
}

// StatsResponse aggregates all types of stats of one container.
type StatsResponse struct {
	// ID is the ID of the container for which the stats were collected.
	ID string `json:"id,omitempty"`

	// Name is the name of the container for which the stats were collected.
	Name string `json:"name,omitempty"`

	// OSType is the OS of the container ("linux" or "windows") to allow
	// platform-specific handling of stats.
	OSType string `json:"os_type,omitempty"`

	// Read is the date and time at which this sample was collected.
	Read time.Time `json:"read"`

	// CPUStats contains CPU related info of the container.
	CPUStats CPUStats `json:"cpu_stats,omitempty"`

	// MemoryStats aggregates all memory stats since container inception on Linux.
	// Windows returns stats for commit and private working set only.
	MemoryStats MemoryStats `json:"memory_stats,omitempty"`

	// Networks contains Nntwork statistics for the container per interface.
	//
	// This field is omitted if the container has no networking enabled.
	Networks map[string]NetworkStats `json:"networks,omitempty"`

	// -------------------------------------------------------------------------
	// Linux-specific stats, not populated on Windows.
	// -------------------------------------------------------------------------

	// PidsStats contains Linux-specific stats of a container's process-IDs (PIDs).
	//
	// This field is Linux-specific and omitted for Windows containers.
	PidsStats PidsStats `json:"pids_stats,omitempty"`

	// BlkioStats stores all IO service stats for data read and write.
	//
	// This type is Linux-specific and holds many fields that are specific
	// to cgroups v1.
	//
	// On a cgroup v2 host, all fields other than "io_service_bytes_recursive"
	// are omitted or "null".
	//
	// This type is only populated on Linux and omitted for Windows containers.
	BlkioStats BlkioStats `json:"blkio_stats,omitempty"`

	// -------------------------------------------------------------------------
	// Windows-specific stats, not populated on Linux.
	// -------------------------------------------------------------------------

	// NumProcs is the number of processors on the system.
	//
	// This field is Windows-specific and always zero for Linux containers.
	NumProcs uint32 `json:"num_procs"`

	// StorageStats is the disk I/O stats for read/write on Windows.
	//
	// This type is Windows-specific and omitted for Linux containers.
	StorageStats StorageStats `json:"storage_stats,omitempty"`

	// -------------------------------------------------------------------------
	// PreRead and PreCPUStats contain the previous sample of stats for
	// the container, and can be used to perform delta-calculation.
	// -------------------------------------------------------------------------

	// PreRead is the date and time at which this first sample was collected.
	// This field is not propagated if the "one-shot" option is set. If the
	// "one-shot" option is set, this field may be omitted, empty, or set
	// to a default date (`0001-01-01T00:00:00Z`).
	PreRead time.Time `json:"preread"`

	// PreCPUStats contains the CPUStats of the previous sample.
	PreCPUStats CPUStats `json:"precpu_stats,omitempty"`
}

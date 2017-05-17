package runtime

import "time"

// Stat holds a container statistics
type Stat struct {
	// Timestamp is the time that the statistics where collected
	Timestamp time.Time
	CPU       CPU                `json:"cpu"`
	Memory    Memory             `json:"memory"`
	Pids      Pids               `json:"pids"`
	Blkio     Blkio              `json:"blkio"`
	Hugetlb   map[string]Hugetlb `json:"hugetlb"`
}

// Hugetlb holds information regarding a container huge tlb usage
type Hugetlb struct {
	Usage   uint64 `json:"usage,omitempty"`
	Max     uint64 `json:"max,omitempty"`
	Failcnt uint64 `json:"failcnt"`
}

// BlkioEntry represents a single record for a Blkio stat
type BlkioEntry struct {
	Major uint64 `json:"major,omitempty"`
	Minor uint64 `json:"minor,omitempty"`
	Op    string `json:"op,omitempty"`
	Value uint64 `json:"value,omitempty"`
}

// Blkio regroups all the Blkio related stats
type Blkio struct {
	IoServiceBytesRecursive []BlkioEntry `json:"ioServiceBytesRecursive,omitempty"`
	IoServicedRecursive     []BlkioEntry `json:"ioServicedRecursive,omitempty"`
	IoQueuedRecursive       []BlkioEntry `json:"ioQueueRecursive,omitempty"`
	IoServiceTimeRecursive  []BlkioEntry `json:"ioServiceTimeRecursive,omitempty"`
	IoWaitTimeRecursive     []BlkioEntry `json:"ioWaitTimeRecursive,omitempty"`
	IoMergedRecursive       []BlkioEntry `json:"ioMergedRecursive,omitempty"`
	IoTimeRecursive         []BlkioEntry `json:"ioTimeRecursive,omitempty"`
	SectorsRecursive        []BlkioEntry `json:"sectorsRecursive,omitempty"`
}

// Pids holds the stat of the pid usage of the machine
type Pids struct {
	Current uint64 `json:"current,omitempty"`
	Limit   uint64 `json:"limit,omitempty"`
}

// Throttling holds a cpu throttling information
type Throttling struct {
	Periods          uint64 `json:"periods,omitempty"`
	ThrottledPeriods uint64 `json:"throttledPeriods,omitempty"`
	ThrottledTime    uint64 `json:"throttledTime,omitempty"`
}

// CPUUsage holds information regarding cpu usage
type CPUUsage struct {
	// Units: nanoseconds.
	Total  uint64   `json:"total,omitempty"`
	Percpu []uint64 `json:"percpu,omitempty"`
	Kernel uint64   `json:"kernel"`
	User   uint64   `json:"user"`
}

// CPU regroups both a CPU usage and throttling information
type CPU struct {
	Usage      CPUUsage   `json:"usage,omitempty"`
	Throttling Throttling `json:"throttling,omitempty"`
}

// MemoryEntry regroups statistic about a given type of memory
type MemoryEntry struct {
	Limit   uint64 `json:"limit"`
	Usage   uint64 `json:"usage,omitempty"`
	Max     uint64 `json:"max,omitempty"`
	Failcnt uint64 `json:"failcnt"`
}

// Memory holds information regarding the different type of memories available
type Memory struct {
	Cache     uint64            `json:"cache,omitempty"`
	Usage     MemoryEntry       `json:"usage,omitempty"`
	Swap      MemoryEntry       `json:"swap,omitempty"`
	Kernel    MemoryEntry       `json:"kernel,omitempty"`
	KernelTCP MemoryEntry       `json:"kernelTCP,omitempty"`
	Raw       map[string]uint64 `json:"raw,omitempty"`
}

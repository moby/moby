package types

import (
	"context"
	"time"
)

type Recorder interface {
	Start()
	Close()
	CloseAsync(func(context.Context) error) error
	Wait() error
	Samples() (*Samples, error)
}

type Samples struct {
	Samples    []*Sample   `json:"samples,omitempty"`
	SysCPUStat *SysCPUStat `json:"sysCPUStat,omitempty"`
}

// Sample represents a wrapper for sampled data of cgroupv2 controllers
type Sample struct {
	//nolint
	Timestamp_ time.Time      `json:"timestamp"`
	CPUStat    *CPUStat       `json:"cpuStat,omitempty"`
	MemoryStat *MemoryStat    `json:"memoryStat,omitempty"`
	IOStat     *IOStat        `json:"ioStat,omitempty"`
	PIDsStat   *PIDsStat      `json:"pidsStat,omitempty"`
	NetStat    *NetworkSample `json:"netStat,omitempty"`
}

func (s *Sample) Timestamp() time.Time {
	return s.Timestamp_
}

type NetworkSample struct {
	RxBytes   int64 `json:"rxBytes,omitempty"`
	RxPackets int64 `json:"rxPackets,omitempty"`
	RxErrors  int64 `json:"rxErrors,omitempty"`
	RxDropped int64 `json:"rxDropped,omitempty"`
	TxBytes   int64 `json:"txBytes,omitempty"`
	TxPackets int64 `json:"txPackets,omitempty"`
	TxErrors  int64 `json:"txErrors,omitempty"`
	TxDropped int64 `json:"txDropped,omitempty"`
}

// CPUStat represents the sampling state of the cgroupv2 CPU controller
type CPUStat struct {
	UsageNanos     *uint64   `json:"usageNanos,omitempty"`
	UserNanos      *uint64   `json:"userNanos,omitempty"`
	SystemNanos    *uint64   `json:"systemNanos,omitempty"`
	NrPeriods      *uint32   `json:"nrPeriods,omitempty"`
	NrThrottled    *uint32   `json:"nrThrottled,omitempty"`
	ThrottledNanos *uint64   `json:"throttledNanos,omitempty"`
	Pressure       *Pressure `json:"pressure,omitempty"`
}

// MemoryStat represents the sampling state of the cgroupv2 memory controller
type MemoryStat struct {
	SwapBytes     *uint64   `json:"swapBytes,omitempty"`
	Anon          *uint64   `json:"anon,omitempty"`
	File          *uint64   `json:"file,omitempty"`
	Kernel        *uint64   `json:"kernel,omitempty"`
	KernelStack   *uint64   `json:"kernelStack,omitempty"`
	PageTables    *uint64   `json:"pageTables,omitempty"`
	Sock          *uint64   `json:"sock,omitempty"`
	Vmalloc       *uint64   `json:"vmalloc,omitempty"`
	Shmem         *uint64   `json:"shmem,omitempty"`
	FileMapped    *uint64   `json:"fileMapped,omitempty"`
	FileDirty     *uint64   `json:"fileDirty,omitempty"`
	FileWriteback *uint64   `json:"fileWriteback,omitempty"`
	Slab          *uint64   `json:"slab,omitempty"`
	Pgscan        *uint64   `json:"pgscan,omitempty"`
	Pgsteal       *uint64   `json:"pgsteal,omitempty"`
	Pgfault       *uint64   `json:"pgfault,omitempty"`
	Pgmajfault    *uint64   `json:"pgmajfault,omitempty"`
	Peak          *uint64   `json:"peak,omitempty"`
	LowEvents     uint64    `json:"lowEvents,omitempty"`
	HighEvents    uint64    `json:"highEvents,omitempty"`
	MaxEvents     uint64    `json:"maxEvents,omitempty"`
	OomEvents     uint64    `json:"oomEvents,omitempty"`
	OomKillEvents uint64    `json:"oomKillEvents,omitempty"`
	Pressure      *Pressure `json:"pressure,omitempty"`
}

// IOStat represents the sampling state of the cgroupv2 IO controller
type IOStat struct {
	ReadBytes    *uint64   `json:"readBytes,omitempty"`
	WriteBytes   *uint64   `json:"writeBytes,omitempty"`
	DiscardBytes *uint64   `json:"discardBytes,omitempty"`
	ReadIOs      *uint64   `json:"readIOs,omitempty"`
	WriteIOs     *uint64   `json:"writeIOs,omitempty"`
	DiscardIOs   *uint64   `json:"discardIOs,omitempty"`
	Pressure     *Pressure `json:"pressure,omitempty"`
}

// PIDsStat represents the sampling state of the cgroupv2 PIDs controller
type PIDsStat struct {
	Current *uint64 `json:"current,omitempty"`
}

// Pressure represents the sampling state of pressure files
type Pressure struct {
	Some *PressureValues `json:"some"`
	Full *PressureValues `json:"full"`
}

type PressureValues struct {
	Avg10  *float64 `json:"avg10"`
	Avg60  *float64 `json:"avg60"`
	Avg300 *float64 `json:"avg300"`
	Total  *uint64  `json:"total"`
}

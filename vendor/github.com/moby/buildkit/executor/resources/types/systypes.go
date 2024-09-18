package types

import (
	"encoding/json"
	"math"
	"time"
)

type SysCPUStat struct {
	User      float64 `json:"user"`
	Nice      float64 `json:"nice"`
	System    float64 `json:"system"`
	Idle      float64 `json:"idle"`
	Iowait    float64 `json:"iowait"`
	IRQ       float64 `json:"irq"`
	SoftIRQ   float64 `json:"softirq"`
	Steal     float64 `json:"steal"`
	Guest     float64 `json:"guest"`
	GuestNice float64 `json:"guestNice"`
}

type sysCPUStatAlias SysCPUStat // avoid recursion of MarshalJSON

func (s SysCPUStat) MarshalJSON() ([]byte, error) {
	return json.Marshal(sysCPUStatAlias{
		User:      math.Round(s.User*1000) / 1000,
		Nice:      math.Round(s.Nice*1000) / 1000,
		System:    math.Round(s.System*1000) / 1000,
		Idle:      math.Round(s.Idle*1000) / 1000,
		Iowait:    math.Round(s.Iowait*1000) / 1000,
		IRQ:       math.Round(s.IRQ*1000) / 1000,
		SoftIRQ:   math.Round(s.SoftIRQ*1000) / 1000,
		Steal:     math.Round(s.Steal*1000) / 1000,
		Guest:     math.Round(s.Guest*1000) / 1000,
		GuestNice: math.Round(s.GuestNice*1000) / 1000,
	})
}

type ProcStat struct {
	ContextSwitches  uint64 `json:"contextSwitches"`
	ProcessCreated   uint64 `json:"processCreated"`
	ProcessesRunning uint64 `json:"processesRunning"`
}

type SysMemoryStat struct {
	Total     *uint64 `json:"total"`
	Free      *uint64 `json:"free"`
	Available *uint64 `json:"available"`
	Buffers   *uint64 `json:"buffers"`
	Cached    *uint64 `json:"cached"`
	Active    *uint64 `json:"active"`
	Inactive  *uint64 `json:"inactive"`
	Swap      *uint64 `json:"swap"`
	Dirty     *uint64 `json:"dirty"`
	Writeback *uint64 `json:"writeback"`
	Slab      *uint64 `json:"slab"`
}

type SysSample struct {
	//nolint
	Timestamp_     time.Time      `json:"timestamp"`
	CPUStat        *SysCPUStat    `json:"cpuStat,omitempty"`
	ProcStat       *ProcStat      `json:"procStat,omitempty"`
	MemoryStat     *SysMemoryStat `json:"memoryStat,omitempty"`
	CPUPressure    *Pressure      `json:"cpuPressure,omitempty"`
	MemoryPressure *Pressure      `json:"memoryPressure,omitempty"`
	IOPressure     *Pressure      `json:"ioPressure,omitempty"`
}

func (s *SysSample) Timestamp() time.Time {
	return s.Timestamp_
}

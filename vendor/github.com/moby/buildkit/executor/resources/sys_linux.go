package resources

import (
	"os"
	"time"

	resourcestypes "github.com/moby/buildkit/executor/resources/types"
	"github.com/prometheus/procfs"
)

func newSysSampler() (*Sampler[*resourcestypes.SysSample], error) {
	pfs, err := procfs.NewDefaultFS()
	if err != nil {
		return nil, err
	}

	return NewSampler(2*time.Second, 20, func(tm time.Time) (*resourcestypes.SysSample, error) {
		return sampleSys(pfs, tm)
	}), nil
}

func sampleSys(proc procfs.FS, tm time.Time) (*resourcestypes.SysSample, error) {
	stat, err := proc.Stat()
	if err != nil {
		return nil, err
	}

	s := &resourcestypes.SysSample{
		Timestamp_: tm,
	}

	s.CPUStat = &resourcestypes.SysCPUStat{
		User:      stat.CPUTotal.User,
		Nice:      stat.CPUTotal.Nice,
		System:    stat.CPUTotal.System,
		Idle:      stat.CPUTotal.Idle,
		Iowait:    stat.CPUTotal.Iowait,
		IRQ:       stat.CPUTotal.IRQ,
		SoftIRQ:   stat.CPUTotal.SoftIRQ,
		Steal:     stat.CPUTotal.Steal,
		Guest:     stat.CPUTotal.Guest,
		GuestNice: stat.CPUTotal.GuestNice,
	}

	s.ProcStat = &resourcestypes.ProcStat{
		ContextSwitches:  stat.ContextSwitches,
		ProcessCreated:   stat.ProcessCreated,
		ProcessesRunning: stat.ProcessesRunning,
	}

	mem, err := proc.Meminfo()
	if err != nil {
		return nil, err
	}

	s.MemoryStat = &resourcestypes.SysMemoryStat{
		Total:     mem.MemTotal,
		Free:      mem.MemFree,
		Buffers:   mem.Buffers,
		Cached:    mem.Cached,
		Active:    mem.Active,
		Inactive:  mem.Inactive,
		Swap:      mem.SwapTotal,
		Available: mem.MemAvailable,
		Dirty:     mem.Dirty,
		Writeback: mem.Writeback,
		Slab:      mem.Slab,
	}

	if _, err := os.Lstat("/proc/pressure"); err != nil {
		return s, nil
	}

	cp, err := parsePressureFile("/proc/pressure/cpu")
	if err != nil {
		return nil, err
	}
	s.CPUPressure = cp

	mp, err := parsePressureFile("/proc/pressure/memory")
	if err != nil {
		return nil, err
	}
	s.MemoryPressure = mp

	ip, err := parsePressureFile("/proc/pressure/io")
	if err != nil {
		return nil, err
	}
	s.IOPressure = ip

	return s, nil
}

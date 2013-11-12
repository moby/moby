package stats

type CpuInfo struct {
	Cpus    int
	Average [3]float64
}

type MemInfo struct {
	Free  uint64
	Total uint64
}

type SysInfo struct {
	CpuInfo CpuInfo
	MemInfo MemInfo
}

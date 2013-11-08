package stats

func NewSysInfo() *SysInfo {
	sysinfo := new(SysInfo)
	sysinfo.CpuInfo.Cpus = runtime.NumCPU()
	return sysinfo
}

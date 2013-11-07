package stats

import (
	"runtime"
	"syscall"
)

func NewSysInfo() *SysInfo {
	var info syscall.Sysinfo_t
	err := syscall.Sysinfo(&info)
	sysinfo := new(SysInfo)
	if err == nil {
		sysinfo.MemInfo = MemInfo{Free: info.Freeram, Total: info.Totalram}
		for i, avg := range info.Loads {
			sysinfo.CpuInfo.Average[i] = float64(avg) / 65536.0
		}
	}
	sysinfo.CpuInfo.Cpus = runtime.NumCPU()
	return sysinfo
}

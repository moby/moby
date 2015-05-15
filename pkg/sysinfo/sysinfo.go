package sysinfo

// SysInfo stores information about which features a kernel supports.
// TODO Windows: Factor out platform specific capabilities.
type SysInfo struct {
	MemoryLimit            bool
	SwapLimit              bool
	CpuCfsPeriod           bool
	CpuCfsQuota            bool
	IPv4ForwardingDisabled bool
	AppArmor               bool
	OomKillDisable         bool
}

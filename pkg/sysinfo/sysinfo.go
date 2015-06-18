package sysinfo

// SysInfo stores information about which features a kernel supports.
// TODO Windows: Factor out platform specific capabilities.
type SysInfo struct {
	AppArmor bool
	*cgroupMemInfo
	*cgroupCpuInfo
	*cgroupBlkioInfo
	*cgroupCpusetInfo
	IPv4ForwardingDisabled        bool
	BridgeNfCallIptablesDisabled  bool
	BridgeNfCallIp6tablesDisabled bool
	CgroupDevicesEnabled          bool
}

type cgroupMemInfo struct {
	MemoryLimit    bool
	SwapLimit      bool
	OomKillDisable bool
}

type cgroupCpuInfo struct {
	CpuShares    bool
	CpuCfsPeriod bool
	CpuCfsQuota  bool
}

type cgroupBlkioInfo struct {
	BlkioWeight bool
}

type cgroupCpusetInfo struct {
	Cpuset bool
}

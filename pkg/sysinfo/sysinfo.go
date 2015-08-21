package sysinfo

// SysInfo stores information about which features a kernel supports.
// TODO Windows: Factor out platform specific capabilities.
type SysInfo struct {
	// Whether the kernel supports AppArmor or not
	AppArmor bool

	cgroupMemInfo
	cgroupCPUInfo
	cgroupBlkioInfo
	cgroupCpusetInfo

	// Whether IPv4 forwarding is supported or not, if this was disabled, networking will not work
	IPv4ForwardingDisabled bool

	// Whether bridge-nf-call-iptables is supported or not
	BridgeNfCallIptablesDisabled bool

	// Whether bridge-nf-call-ip6tables is supported or not
	BridgeNfCallIP6tablesDisabled bool

	// Whether the cgroup has the mountpoint of "devices" or not
	CgroupDevicesEnabled bool
}

type cgroupMemInfo struct {
	// Whether memory limit is supported or not
	MemoryLimit bool

	// Whether swap limit is supported or not
	SwapLimit bool

	// Whether OOM killer disalbe is supported or not
	OomKillDisable bool

	// Whether memory swappiness is supported or not
	MemorySwappiness bool

	// Whether kernel memory limit is supported or not
	KernelMemory bool
}

type cgroupCPUInfo struct {
	// Whether CPU shares is supported or not
	CPUShares bool

	// Whether CPU CFS(Completely Fair Scheduler) period is supported or not
	CPUCfsPeriod bool

	// Whether CPU CFS(Completely Fair Scheduler) quota is supported or not
	CPUCfsQuota bool
}

type cgroupBlkioInfo struct {
	// Whether Block IO weight is supported or not
	BlkioWeight bool
}

type cgroupCpusetInfo struct {
	// Whether Cpuset is supported or not
	Cpuset bool
}

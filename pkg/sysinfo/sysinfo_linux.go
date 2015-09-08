package sysinfo

import (
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/opencontainers/runc/libcontainer/cgroups"
)

// New returns a new SysInfo, using the filesystem to detect which features
// the kernel supports. If `quiet` is `false` warnings are printed in logs
// whenever an error occurs or misconfigurations are present.
func New(quiet bool) *SysInfo {
	sysInfo := &SysInfo{}
	sysInfo.cgroupMemInfo = checkCgroupMem(quiet)
	sysInfo.cgroupCPUInfo = checkCgroupCPU(quiet)
	sysInfo.cgroupBlkioInfo = checkCgroupBlkioInfo(quiet)
	sysInfo.cgroupCpusetInfo = checkCgroupCpusetInfo(quiet)

	_, err := cgroups.FindCgroupMountpoint("devices")
	sysInfo.CgroupDevicesEnabled = err == nil

	sysInfo.IPv4ForwardingDisabled = !readProcBool("/proc/sys/net/ipv4/ip_forward")
	sysInfo.BridgeNfCallIptablesDisabled = !readProcBool("/proc/sys/net/bridge/bridge-nf-call-iptables")
	sysInfo.BridgeNfCallIP6tablesDisabled = !readProcBool("/proc/sys/net/bridge/bridge-nf-call-ip6tables")

	// Check if AppArmor is supported.
	if _, err := os.Stat("/sys/kernel/security/apparmor"); !os.IsNotExist(err) {
		sysInfo.AppArmor = true
	}

	return sysInfo
}

// checkCgroupMem reads the memory information from the memory cgroup mount point.
func checkCgroupMem(quiet bool) cgroupMemInfo {
	mountPoint, err := cgroups.FindCgroupMountpoint("memory")
	if err != nil {
		if !quiet {
			logrus.Warnf("Your kernel does not support cgroup memory limit: %v", err)
		}
		return cgroupMemInfo{}
	}

	swapLimit := cgroupEnabled(mountPoint, "memory.memsw.limit_in_bytes")
	if !quiet && !swapLimit {
		logrus.Warn("Your kernel does not support swap memory limit.")
	}
	memoryReservation := cgroupEnabled(mountPoint, "memory.soft_limit_in_bytes")
	if !quiet && !memoryReservation {
		logrus.Warn("Your kernel does not support memory reservation.")
	}
	oomKillDisable := cgroupEnabled(mountPoint, "memory.oom_control")
	if !quiet && !oomKillDisable {
		logrus.Warnf("Your kernel does not support oom control.")
	}
	memorySwappiness := cgroupEnabled(mountPoint, "memory.swappiness")
	if !quiet && !memorySwappiness {
		logrus.Warnf("Your kernel does not support memory swappiness.")
	}
	kernelMemory := cgroupEnabled(mountPoint, "memory.kmem.limit_in_bytes")
	if !quiet && !kernelMemory {
		logrus.Warnf("Your kernel does not support kernel memory limit.")
	}

	return cgroupMemInfo{
		MemoryLimit:       true,
		SwapLimit:         swapLimit,
		MemoryReservation: memoryReservation,
		OomKillDisable:    oomKillDisable,
		MemorySwappiness:  memorySwappiness,
		KernelMemory:      kernelMemory,
	}
}

// checkCgroupCPU reads the cpu information from the cpu cgroup mount point.
func checkCgroupCPU(quiet bool) cgroupCPUInfo {
	mountPoint, err := cgroups.FindCgroupMountpoint("cpu")
	if err != nil {
		if !quiet {
			logrus.Warn(err)
		}
		return cgroupCPUInfo{}
	}

	cpuShares := cgroupEnabled(mountPoint, "cpu.shares")
	if !quiet && !cpuShares {
		logrus.Warn("Your kernel does not support cgroup cpu shares")
	}

	cpuCfsPeriod := cgroupEnabled(mountPoint, "cpu.cfs_period_us")
	if !quiet && !cpuCfsPeriod {
		logrus.Warn("Your kernel does not support cgroup cfs period")
	}

	cpuCfsQuota := cgroupEnabled(mountPoint, "cpu.cfs_quota_us")
	if !quiet && !cpuCfsQuota {
		logrus.Warn("Your kernel does not support cgroup cfs quotas")
	}
	return cgroupCPUInfo{
		CPUShares:    cpuShares,
		CPUCfsPeriod: cpuCfsPeriod,
		CPUCfsQuota:  cpuCfsQuota,
	}
}

// checkCgroupBlkioInfo reads the blkio information from the blkio cgroup mount point.
func checkCgroupBlkioInfo(quiet bool) cgroupBlkioInfo {
	mountPoint, err := cgroups.FindCgroupMountpoint("blkio")
	if err != nil {
		if !quiet {
			logrus.Warn(err)
		}
		return cgroupBlkioInfo{}
	}

	w := cgroupEnabled(mountPoint, "blkio.weight")
	if !quiet && !w {
		logrus.Warn("Your kernel does not support cgroup blkio weight")
	}
	return cgroupBlkioInfo{BlkioWeight: w}
}

// checkCgroupCpusetInfo reads the cpuset information from the cpuset cgroup mount point.
func checkCgroupCpusetInfo(quiet bool) cgroupCpusetInfo {
	mountPoint, err := cgroups.FindCgroupMountpoint("cpuset")
	if err != nil {
		if !quiet {
			logrus.Warn(err)
		}
		return cgroupCpusetInfo{}
	}

	cpus, err := ioutil.ReadFile(path.Join(mountPoint, "cpuset.cpus"))
	if err != nil {
		return cgroupCpusetInfo{}
	}

	mems, err := ioutil.ReadFile(path.Join(mountPoint, "cpuset.mems"))
	if err != nil {
		return cgroupCpusetInfo{}
	}

	return cgroupCpusetInfo{
		Cpuset: true,
		Cpus:   strings.TrimSpace(string(cpus)),
		Mems:   strings.TrimSpace(string(mems)),
	}
}

func cgroupEnabled(mountPoint, name string) bool {
	_, err := os.Stat(path.Join(mountPoint, name))
	return err == nil
}

func readProcBool(path string) bool {
	val, err := ioutil.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(val)) == "1"
}

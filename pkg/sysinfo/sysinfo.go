package sysinfo

import (
	"io/ioutil"
	"os"
	"path"

	"github.com/Sirupsen/logrus"
	"github.com/docker/libcontainer/cgroups"
)

// SysInfo stores information about which features a kernel supports.
type SysInfo struct {
	MemoryLimit            bool
	SwapLimit              bool
	CpuCfsQuota            bool
	IPv4ForwardingDisabled bool
	AppArmor               bool
}

// New returns a new SysInfo, using the filesystem to detect which features the kernel supports.
func New(quiet bool) *SysInfo {
	sysInfo := &SysInfo{}
	if cgroupMemoryMountpoint, err := cgroups.FindCgroupMountpoint("memory"); err != nil {
		if !quiet {
			logrus.Warnf("%v", err)
		}
	} else {
		_, err1 := ioutil.ReadFile(path.Join(cgroupMemoryMountpoint, "memory.limit_in_bytes"))
		_, err2 := ioutil.ReadFile(path.Join(cgroupMemoryMountpoint, "memory.soft_limit_in_bytes"))
		sysInfo.MemoryLimit = err1 == nil && err2 == nil
		if !sysInfo.MemoryLimit && !quiet {
			logrus.Warn("Your kernel does not support cgroup memory limit.")
		}

		_, err = ioutil.ReadFile(path.Join(cgroupMemoryMountpoint, "memory.memsw.limit_in_bytes"))
		sysInfo.SwapLimit = err == nil
		if !sysInfo.SwapLimit && !quiet {
			logrus.Warn("Your kernel does not support cgroup swap limit.")
		}
	}

	if cgroupCpuMountpoint, err := cgroups.FindCgroupMountpoint("cpu"); err != nil {
		if !quiet {
			logrus.Warnf("%v", err)
		}
	} else {
		_, err1 := ioutil.ReadFile(path.Join(cgroupCpuMountpoint, "cpu.cfs_quota_us"))
		sysInfo.CpuCfsQuota = err1 == nil
		if !sysInfo.CpuCfsQuota && !quiet {
			logrus.Warn("Your kernel does not support cgroup cfs quotas")
		}
	}

	// Check if AppArmor is supported.
	if _, err := os.Stat("/sys/kernel/security/apparmor"); os.IsNotExist(err) {
		sysInfo.AppArmor = false
	} else {
		sysInfo.AppArmor = true
	}
	return sysInfo
}

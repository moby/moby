package sysinfo

import (
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"

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
			logrus.Warnf("Your kernel does not support cgroup memory limit: %v", err)
		}
	} else {
		// If memory cgroup is mounted, MemoryLimit is always enabled.
		sysInfo.MemoryLimit = true

		_, err1 := ioutil.ReadFile(path.Join(cgroupMemoryMountpoint, "memory.memsw.limit_in_bytes"))
		sysInfo.SwapLimit = err1 == nil
		if !sysInfo.SwapLimit && !quiet {
			logrus.Warn("Your kernel does not support swap memory limit.")
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

	// Checek if ipv4_forward is disabled.
	if data, err := ioutil.ReadFile("/proc/sys/net/ipv4/ip_forward"); os.IsNotExist(err) {
		sysInfo.IPv4ForwardingDisabled = true
	} else {
		if enabled, _ := strconv.Atoi(strings.TrimSpace(string(data))); enabled == 0 {
			sysInfo.IPv4ForwardingDisabled = true
		} else {
			sysInfo.IPv4ForwardingDisabled = false
		}
	}

	// Check if AppArmor is supported.
	if _, err := os.Stat("/sys/kernel/security/apparmor"); os.IsNotExist(err) {
		sysInfo.AppArmor = false
	} else {
		sysInfo.AppArmor = true
	}

	// Check if Devices cgroup is mounted, it is hard requirement for container security.
	if _, err := cgroups.FindCgroupMountpoint("devices"); err != nil {
		logrus.Fatalf("Error mounting devices cgroup: %v", err)
	}

	return sysInfo
}

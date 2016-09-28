// +build !windows

package main

import (
	"io/ioutil"
	"strings"

	"github.com/docker/docker/pkg/sysinfo"
)

var (
	// SysInfo stores information about which features a kernel supports.
	SysInfo      *sysinfo.SysInfo
	cpuCfsPeriod = testRequirement{
		func() bool {
			return SysInfo.CPUCfsPeriod
		},
		"Test requires an environment that supports cgroup cfs period.",
	}
	cpuCfsQuota = testRequirement{
		func() bool {
			return SysInfo.CPUCfsQuota
		},
		"Test requires an environment that supports cgroup cfs quota.",
	}
	cpuShare = testRequirement{
		func() bool {
			return SysInfo.CPUShares
		},
		"Test requires an environment that supports cgroup cpu shares.",
	}
	oomControl = testRequirement{
		func() bool {
			return SysInfo.OomKillDisable
		},
		"Test requires Oom control enabled.",
	}
	pidsLimit = testRequirement{
		func() bool {
			return SysInfo.PidsLimit
		},
		"Test requires pids limit enabled.",
	}
	kernelMemorySupport = testRequirement{
		func() bool {
			return SysInfo.KernelMemory
		},
		"Test requires an environment that supports cgroup kernel memory.",
	}
	memoryLimitSupport = testRequirement{
		func() bool {
			return SysInfo.MemoryLimit
		},
		"Test requires an environment that supports cgroup memory limit.",
	}
	memoryReservationSupport = testRequirement{
		func() bool {
			return SysInfo.MemoryReservation
		},
		"Test requires an environment that supports cgroup memory reservation.",
	}
	swapMemorySupport = testRequirement{
		func() bool {
			return SysInfo.SwapLimit
		},
		"Test requires an environment that supports cgroup swap memory limit.",
	}
	memorySwappinessSupport = testRequirement{
		func() bool {
			return SysInfo.MemorySwappiness
		},
		"Test requires an environment that supports cgroup memory swappiness.",
	}
	blkioWeight = testRequirement{
		func() bool {
			return SysInfo.BlkioWeight
		},
		"Test requires an environment that supports blkio weight.",
	}
	cgroupCpuset = testRequirement{
		func() bool {
			return SysInfo.Cpuset
		},
		"Test requires an environment that supports cgroup cpuset.",
	}
	seccompEnabled = testRequirement{
		func() bool {
			return supportsSeccomp && SysInfo.Seccomp
		},
		"Test requires that seccomp support be enabled in the daemon.",
	}
	bridgeNfIptables = testRequirement{
		func() bool {
			return !SysInfo.BridgeNFCallIPTablesDisabled
		},
		"Test requires that bridge-nf-call-iptables support be enabled in the daemon.",
	}
	bridgeNfIP6tables = testRequirement{
		func() bool {
			return !SysInfo.BridgeNFCallIP6TablesDisabled
		},
		"Test requires that bridge-nf-call-ip6tables support be enabled in the daemon.",
	}
	unprivilegedUsernsClone = testRequirement{
		func() bool {
			content, err := ioutil.ReadFile("/proc/sys/kernel/unprivileged_userns_clone")
			if err == nil && strings.Contains(string(content), "0") {
				return false
			}
			return true
		},
		"Test cannot be run with 'sysctl kernel.unprivileged_userns_clone' = 0",
	}
	ambientCapabilities = testRequirement{
		func() bool {
			content, err := ioutil.ReadFile("/proc/self/status")
			if err == nil && strings.Contains(string(content), "CapAmb:") {
				return true
			}
			return false
		},
		"Test cannot be run without a kernel (4.3+) supporting ambient capabilities",
	}
)

func init() {
	SysInfo = sysinfo.New(true)
}

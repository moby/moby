//go:build !windows

package main

import (
	"os"
	"strings"

	"github.com/containerd/cgroups/v3"
	"github.com/moby/moby/v2/pkg/sysinfo"
)

var sysInfo *sysinfo.SysInfo

func setupLocalInfo() {
	sysInfo = sysinfo.New()
}

func cpuCfsPeriod() bool {
	return testEnv.DaemonInfo.CPUCfsPeriod
}

func cpuCfsQuota() bool {
	return testEnv.DaemonInfo.CPUCfsQuota
}

func cpuShare() bool {
	return testEnv.DaemonInfo.CPUShares
}

func oomControl() bool {
	return testEnv.DaemonInfo.OomKillDisable
}

func pidsLimit() bool {
	return sysInfo.PidsLimit
}

func memoryLimitSupport() bool {
	return testEnv.DaemonInfo.MemoryLimit
}

func memoryReservationSupport() bool {
	return sysInfo.MemoryReservation
}

func swapMemorySupport() bool {
	return testEnv.DaemonInfo.SwapLimit
}

func memorySwappinessSupport() bool {
	return testEnv.IsLocalDaemon() && sysInfo.MemorySwappiness
}

func blkioWeight() bool {
	return testEnv.IsLocalDaemon() && sysInfo.BlkioWeight
}

func cgroupCpuset() bool {
	return testEnv.DaemonInfo.CPUSet
}

func seccompEnabled() bool {
	return sysInfo.Seccomp
}

func onlyCgroupsv2() bool {
	// Only check for unified, cgroup v1 tests can run under other modes
	return cgroups.Mode() == cgroups.Unified
}

func unprivilegedUsernsClone() bool {
	content, err := os.ReadFile("/proc/sys/kernel/unprivileged_userns_clone")
	return err != nil || !strings.Contains(string(content), "0")
}

package sysinfo // import "github.com/docker/docker/pkg/sysinfo"

import (
	"fmt"
	"os"
	"path"
	"strings"

	cdcgroups "github.com/containerd/cgroups"
	cdseccomp "github.com/containerd/containerd/pkg/seccomp"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/sirupsen/logrus"
)

func findCgroupMountpoints() (map[string]string, error) {
	cgMounts, err := cgroups.GetCgroupMounts(false)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse cgroup information: %v", err)
	}
	mps := make(map[string]string)
	for _, m := range cgMounts {
		for _, ss := range m.Subsystems {
			mps[ss] = m.Mountpoint
		}
	}
	return mps, nil
}

type infoCollector func(info *SysInfo)

// WithCgroup2GroupPath specifies the cgroup v2 group path to inspect availability
// of the controllers.
//
// WithCgroup2GroupPath is expected to be used for rootless mode with systemd driver.
//
// e.g. g = "/user.slice/user-1000.slice/user@1000.service"
func WithCgroup2GroupPath(g string) Opt {
	return func(o *SysInfo) {
		if p := path.Clean(g); p != "" {
			o.cg2GroupPath = p
		}
	}
}

// New returns a new SysInfo, using the filesystem to detect which features
// the kernel supports.
func New(options ...Opt) *SysInfo {
	if cdcgroups.Mode() == cdcgroups.Unified {
		return newV2(options...)
	}
	return newV1()
}

func newV1() *SysInfo {
	var (
		err     error
		sysInfo = &SysInfo{}
	)

	ops := []infoCollector{
		applyNetworkingInfo,
		applyAppArmorInfo,
		applySeccompInfo,
		applyCgroupNsInfo,
	}

	sysInfo.cgMounts, err = findCgroupMountpoints()
	if err != nil {
		logrus.Warn(err)
	} else {
		ops = append(ops,
			applyMemoryCgroupInfo,
			applyCPUCgroupInfo,
			applyBlkioCgroupInfo,
			applyCPUSetCgroupInfo,
			applyPIDSCgroupInfo,
			applyDevicesCgroupInfo,
		)
	}

	for _, o := range ops {
		o(sysInfo)
	}
	return sysInfo
}

// applyMemoryCgroupInfo adds the memory cgroup controller information to the info.
func applyMemoryCgroupInfo(info *SysInfo) {
	mountPoint, ok := info.cgMounts["memory"]
	if !ok {
		info.Warnings = append(info.Warnings, "Your kernel does not support cgroup memory limit")
		return
	}
	info.MemoryLimit = ok

	info.SwapLimit = cgroupEnabled(mountPoint, "memory.memsw.limit_in_bytes")
	if !info.SwapLimit {
		info.Warnings = append(info.Warnings, "Your kernel does not support swap memory limit")
	}
	info.MemoryReservation = cgroupEnabled(mountPoint, "memory.soft_limit_in_bytes")
	if !info.MemoryReservation {
		info.Warnings = append(info.Warnings, "Your kernel does not support memory reservation")
	}
	info.OomKillDisable = cgroupEnabled(mountPoint, "memory.oom_control")
	if !info.OomKillDisable {
		info.Warnings = append(info.Warnings, "Your kernel does not support oom control")
	}
	info.MemorySwappiness = cgroupEnabled(mountPoint, "memory.swappiness")
	if !info.MemorySwappiness {
		info.Warnings = append(info.Warnings, "Your kernel does not support memory swappiness")
	}
	info.KernelMemory = cgroupEnabled(mountPoint, "memory.kmem.limit_in_bytes")
	if !info.KernelMemory {
		info.Warnings = append(info.Warnings, "Your kernel does not support kernel memory limit")
	}
	info.KernelMemoryTCP = cgroupEnabled(mountPoint, "memory.kmem.tcp.limit_in_bytes")
	if !info.KernelMemoryTCP {
		info.Warnings = append(info.Warnings, "Your kernel does not support kernel memory TCP limit")
	}
}

// applyCPUCgroupInfo adds the cpu cgroup controller information to the info.
func applyCPUCgroupInfo(info *SysInfo) {
	mountPoint, ok := info.cgMounts["cpu"]
	if !ok {
		info.Warnings = append(info.Warnings, "Unable to find cpu cgroup in mounts")
		return
	}

	info.CPUShares = cgroupEnabled(mountPoint, "cpu.shares")
	if !info.CPUShares {
		info.Warnings = append(info.Warnings, "Your kernel does not support CPU shares")
	}

	info.CPUCfs = cgroupEnabled(mountPoint, "cpu.cfs_quota_us")
	if !info.CPUCfs {
		info.Warnings = append(info.Warnings, "Your kernel does not support CPU CFS scheduler")
	}

	info.CPURealtime = cgroupEnabled(mountPoint, "cpu.rt_period_us")
	if !info.CPURealtime {
		info.Warnings = append(info.Warnings, "Your kernel does not support CPU realtime scheduler")
	}
}

// applyBlkioCgroupInfo adds the blkio cgroup controller information to the info.
func applyBlkioCgroupInfo(info *SysInfo) {
	mountPoint, ok := info.cgMounts["blkio"]
	if !ok {
		info.Warnings = append(info.Warnings, "Unable to find blkio cgroup in mounts")
		return
	}

	info.BlkioWeight = cgroupEnabled(mountPoint, "blkio.weight")
	if !info.BlkioWeight {
		info.Warnings = append(info.Warnings, "Your kernel does not support cgroup blkio weight")
	}

	info.BlkioWeightDevice = cgroupEnabled(mountPoint, "blkio.weight_device")
	if !info.BlkioWeightDevice {
		info.Warnings = append(info.Warnings, "Your kernel does not support cgroup blkio weight_device")
	}

	info.BlkioReadBpsDevice = cgroupEnabled(mountPoint, "blkio.throttle.read_bps_device")
	if !info.BlkioReadBpsDevice {
		info.Warnings = append(info.Warnings, "Your kernel does not support cgroup blkio throttle.read_bps_device")
	}

	info.BlkioWriteBpsDevice = cgroupEnabled(mountPoint, "blkio.throttle.write_bps_device")
	if !info.BlkioWriteBpsDevice {
		info.Warnings = append(info.Warnings, "Your kernel does not support cgroup blkio throttle.write_bps_device")
	}
	info.BlkioReadIOpsDevice = cgroupEnabled(mountPoint, "blkio.throttle.read_iops_device")
	if !info.BlkioReadIOpsDevice {
		info.Warnings = append(info.Warnings, "Your kernel does not support cgroup blkio throttle.read_iops_device")
	}

	info.BlkioWriteIOpsDevice = cgroupEnabled(mountPoint, "blkio.throttle.write_iops_device")
	if !info.BlkioWriteIOpsDevice {
		info.Warnings = append(info.Warnings, "Your kernel does not support cgroup blkio throttle.write_iops_device")
	}
}

// applyCPUSetCgroupInfo adds the cpuset cgroup controller information to the info.
func applyCPUSetCgroupInfo(info *SysInfo) {
	mountPoint, ok := info.cgMounts["cpuset"]
	if !ok {
		info.Warnings = append(info.Warnings, "Unable to find cpuset cgroup in mounts")
		return
	}
	info.Cpuset = ok

	var err error

	cpus, err := os.ReadFile(path.Join(mountPoint, "cpuset.cpus"))
	if err != nil {
		return
	}
	info.Cpus = strings.TrimSpace(string(cpus))

	mems, err := os.ReadFile(path.Join(mountPoint, "cpuset.mems"))
	if err != nil {
		return
	}
	info.Mems = strings.TrimSpace(string(mems))
}

// applyPIDSCgroupInfo adds whether the pids cgroup controller is available to the info.
func applyPIDSCgroupInfo(info *SysInfo) {
	_, ok := info.cgMounts["pids"]
	if !ok {
		info.Warnings = append(info.Warnings, "Unable to find pids cgroup in mounts")
		return
	}
	info.PidsLimit = true
}

// applyDevicesCgroupInfo adds whether the devices cgroup controller is available to the info.
func applyDevicesCgroupInfo(info *SysInfo) {
	_, ok := info.cgMounts["devices"]
	info.CgroupDevicesEnabled = ok
}

// applyNetworkingInfo adds networking information to the info.
func applyNetworkingInfo(info *SysInfo) {
	info.IPv4ForwardingDisabled = !readProcBool("/proc/sys/net/ipv4/ip_forward")
	info.BridgeNFCallIPTablesDisabled = !readProcBool("/proc/sys/net/bridge/bridge-nf-call-iptables")
	info.BridgeNFCallIP6TablesDisabled = !readProcBool("/proc/sys/net/bridge/bridge-nf-call-ip6tables")
}

// applyAppArmorInfo adds whether AppArmor is enabled to the info.
func applyAppArmorInfo(info *SysInfo) {
	if _, err := os.Stat("/sys/kernel/security/apparmor"); !os.IsNotExist(err) {
		if _, err := os.ReadFile("/sys/kernel/security/apparmor/profiles"); err == nil {
			info.AppArmor = true
		}
	}
}

// applyCgroupNsInfo adds whether cgroupns is enabled to the info.
func applyCgroupNsInfo(info *SysInfo) {
	if _, err := os.Stat("/proc/self/ns/cgroup"); !os.IsNotExist(err) {
		info.CgroupNamespaces = true
	}
}

// applySeccompInfo checks if Seccomp is supported, via CONFIG_SECCOMP.
func applySeccompInfo(info *SysInfo) {
	info.Seccomp = cdseccomp.IsEnabled()
}

func cgroupEnabled(mountPoint, name string) bool {
	_, err := os.Stat(path.Join(mountPoint, name))
	return err == nil
}

func readProcBool(path string) bool {
	val, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(val)) == "1"
}

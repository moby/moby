package sysinfo // import "github.com/docker/docker/pkg/sysinfo"

import (
	"fmt"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/containerd/cgroups/v3"
	"github.com/containerd/cgroups/v3/cgroup1"
	"github.com/containerd/containerd/pkg/seccomp"
	"github.com/moby/sys/mountinfo"
	"github.com/sirupsen/logrus"
)

var (
	readMountinfoOnce sync.Once
	readMountinfoErr  error
	cgroupMountinfo   []*mountinfo.Info
)

// readCgroupMountinfo returns a list of cgroup v1 mounts (i.e. the ones
// with fstype of "cgroup") for the current running process.
//
// The results are cached (to avoid re-reading mountinfo which is relatively
// expensive), so it is assumed that cgroup mounts are not being changed.
func readCgroupMountinfo() ([]*mountinfo.Info, error) {
	readMountinfoOnce.Do(func() {
		cgroupMountinfo, readMountinfoErr = mountinfo.GetMounts(
			mountinfo.FSTypeFilter("cgroup"),
		)
	})

	return cgroupMountinfo, readMountinfoErr
}

func findCgroupV1Mountpoints() (map[string]string, error) {
	mounts, err := readCgroupMountinfo()
	if err != nil {
		return nil, err
	}

	allSubsystems, err := cgroup1.ParseCgroupFile("/proc/self/cgroup")
	if err != nil {
		return nil, fmt.Errorf("Failed to parse cgroup information: %v", err)
	}

	allMap := make(map[string]bool)
	for s := range allSubsystems {
		allMap[s] = false
	}

	mps := make(map[string]string)
	for _, mi := range mounts {
		for _, opt := range strings.Split(mi.VFSOptions, ",") {
			seen, known := allMap[opt]
			if known && !seen {
				allMap[opt] = true
				mps[strings.TrimPrefix(opt, "name=")] = mi.Mountpoint
			}
		}
		if len(mps) >= len(allMap) {
			break
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
	if cgroups.Mode() == cgroups.Unified {
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

	sysInfo.cgMounts, err = findCgroupV1Mountpoints()
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

	// Option is deprecated, but still accepted on API < v1.42 with cgroups v1,
	// so setting the field to allow feature detection.
	info.KernelMemory = cgroupEnabled(mountPoint, "memory.kmem.limit_in_bytes")

	// Option is deprecated in runc, but still accepted in our API, so setting
	// the field to allow feature detection, but don't warn if it's missing, to
	// make the daemon logs a bit less noisy.
	info.KernelMemoryTCP = cgroupEnabled(mountPoint, "memory.kmem.tcp.limit_in_bytes")
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
	info.Seccomp = seccomp.IsEnabled()
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

//go:build linux || freebsd
// +build linux freebsd

package daemon // import "github.com/docker/docker/daemon"

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/containerd/cgroups/v3"
	"github.com/containerd/containerd/pkg/userns"
	"github.com/docker/docker/api/types/blkiodev"
	pblkiodev "github.com/docker/docker/api/types/blkiodev"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/daemon/initlayer"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/libcontainerd/remote"
	"github.com/docker/docker/libnetwork"
	nwconfig "github.com/docker/docker/libnetwork/config"
	"github.com/docker/docker/libnetwork/drivers/bridge"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/options"
	lntypes "github.com/docker/docker/libnetwork/types"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/parsers/kernel"
	"github.com/docker/docker/pkg/sysinfo"
	"github.com/docker/docker/runconfig"
	volumemounts "github.com/docker/docker/volume/mounts"
	"github.com/moby/sys/mount"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/selinux/go-selinux"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

const (
	isWindows = false

	// See https://git.kernel.org/cgit/linux/kernel/git/tip/tip.git/tree/kernel/sched/sched.h?id=8cd9234c64c584432f6992fe944ca9e46ca8ea76#n269
	linuxMinCPUShares = 2
	linuxMaxCPUShares = 262144
	// It's not kernel limit, we want this 6M limit to account for overhead during startup, and to supply a reasonable functional container
	linuxMinMemory = 6291456
	// constants for remapped root settings
	defaultIDSpecifier = "default"
	defaultRemappedID  = "dockremap"

	// constant for cgroup drivers
	cgroupFsDriver      = "cgroupfs"
	cgroupSystemdDriver = "systemd"
	cgroupNoneDriver    = "none"
)

type containerGetter interface {
	GetContainer(string) (*container.Container, error)
}

func getMemoryResources(config containertypes.Resources) *specs.LinuxMemory {
	memory := specs.LinuxMemory{}

	if config.Memory > 0 {
		memory.Limit = &config.Memory
	}

	if config.MemoryReservation > 0 {
		memory.Reservation = &config.MemoryReservation
	}

	if config.MemorySwap > 0 {
		memory.Swap = &config.MemorySwap
	}

	if config.MemorySwappiness != nil {
		swappiness := uint64(*config.MemorySwappiness)
		memory.Swappiness = &swappiness
	}

	if config.OomKillDisable != nil {
		memory.DisableOOMKiller = config.OomKillDisable
	}

	if config.KernelMemory != 0 {
		memory.Kernel = &config.KernelMemory
	}

	if config.KernelMemoryTCP != 0 {
		memory.KernelTCP = &config.KernelMemoryTCP
	}

	return &memory
}

func getPidsLimit(config containertypes.Resources) *specs.LinuxPids {
	if config.PidsLimit == nil {
		return nil
	}
	if *config.PidsLimit <= 0 {
		// docker API allows 0 and negative values to unset this to be consistent
		// with default values. When updating values, runc requires -1 to unset
		// the previous limit.
		return &specs.LinuxPids{Limit: -1}
	}
	return &specs.LinuxPids{Limit: *config.PidsLimit}
}

func getCPUResources(config containertypes.Resources) (*specs.LinuxCPU, error) {
	cpu := specs.LinuxCPU{}

	if config.CPUShares < 0 {
		return nil, fmt.Errorf("shares: invalid argument")
	}
	if config.CPUShares >= 0 {
		shares := uint64(config.CPUShares)
		cpu.Shares = &shares
	}

	if config.CpusetCpus != "" {
		cpu.Cpus = config.CpusetCpus
	}

	if config.CpusetMems != "" {
		cpu.Mems = config.CpusetMems
	}

	if config.NanoCPUs > 0 {
		// https://www.kernel.org/doc/Documentation/scheduler/sched-bwc.txt
		period := uint64(100 * time.Millisecond / time.Microsecond)
		quota := config.NanoCPUs * int64(period) / 1e9
		cpu.Period = &period
		cpu.Quota = &quota
	}

	if config.CPUPeriod != 0 {
		period := uint64(config.CPUPeriod)
		cpu.Period = &period
	}

	if config.CPUQuota != 0 {
		q := config.CPUQuota
		cpu.Quota = &q
	}

	if config.CPURealtimePeriod != 0 {
		period := uint64(config.CPURealtimePeriod)
		cpu.RealtimePeriod = &period
	}

	if config.CPURealtimeRuntime != 0 {
		c := config.CPURealtimeRuntime
		cpu.RealtimeRuntime = &c
	}

	return &cpu, nil
}

func getBlkioWeightDevices(config containertypes.Resources) ([]specs.LinuxWeightDevice, error) {
	var stat unix.Stat_t
	var blkioWeightDevices []specs.LinuxWeightDevice

	for _, weightDevice := range config.BlkioWeightDevice {
		if err := unix.Stat(weightDevice.Path, &stat); err != nil {
			return nil, errors.WithStack(&os.PathError{Op: "stat", Path: weightDevice.Path, Err: err})
		}
		weight := weightDevice.Weight
		d := specs.LinuxWeightDevice{Weight: &weight}
		// The type is 32bit on mips.
		d.Major = int64(unix.Major(uint64(stat.Rdev))) //nolint: unconvert
		d.Minor = int64(unix.Minor(uint64(stat.Rdev))) //nolint: unconvert
		blkioWeightDevices = append(blkioWeightDevices, d)
	}

	return blkioWeightDevices, nil
}

func (daemon *Daemon) parseSecurityOpt(securityOptions *container.SecurityOptions, hostConfig *containertypes.HostConfig) error {
	securityOptions.NoNewPrivileges = daemon.configStore.NoNewPrivileges
	return parseSecurityOpt(securityOptions, hostConfig)
}

func parseSecurityOpt(securityOptions *container.SecurityOptions, config *containertypes.HostConfig) error {
	var (
		labelOpts []string
		err       error
	)

	for _, opt := range config.SecurityOpt {
		if opt == "no-new-privileges" {
			securityOptions.NoNewPrivileges = true
			continue
		}
		if opt == "disable" {
			labelOpts = append(labelOpts, "disable")
			continue
		}

		var k, v string
		var ok bool
		if strings.Contains(opt, "=") {
			k, v, ok = strings.Cut(opt, "=")
		} else if strings.Contains(opt, ":") {
			k, v, ok = strings.Cut(opt, ":")
			logrus.Warn("Security options with `:` as a separator are deprecated and will be completely unsupported in 17.04, use `=` instead.")
		}
		if !ok {
			return fmt.Errorf("invalid --security-opt 1: %q", opt)
		}

		switch k {
		case "label":
			labelOpts = append(labelOpts, v)
		case "apparmor":
			securityOptions.AppArmorProfile = v
		case "seccomp":
			securityOptions.SeccompProfile = v
		case "no-new-privileges":
			noNewPrivileges, err := strconv.ParseBool(v)
			if err != nil {
				return fmt.Errorf("invalid --security-opt 2: %q", opt)
			}
			securityOptions.NoNewPrivileges = noNewPrivileges
		default:
			return fmt.Errorf("invalid --security-opt 2: %q", opt)
		}
	}

	securityOptions.ProcessLabel, securityOptions.MountLabel, err = label.InitLabels(labelOpts)
	return err
}

func getBlkioThrottleDevices(devs []*blkiodev.ThrottleDevice) ([]specs.LinuxThrottleDevice, error) {
	var throttleDevices []specs.LinuxThrottleDevice
	var stat unix.Stat_t

	for _, d := range devs {
		if err := unix.Stat(d.Path, &stat); err != nil {
			return nil, errors.WithStack(&os.PathError{Op: "stat", Path: d.Path, Err: err})
		}
		d := specs.LinuxThrottleDevice{Rate: d.Rate}
		// the type is 32bit on mips
		d.Major = int64(unix.Major(uint64(stat.Rdev))) //nolint: unconvert
		d.Minor = int64(unix.Minor(uint64(stat.Rdev))) //nolint: unconvert
		throttleDevices = append(throttleDevices, d)
	}

	return throttleDevices, nil
}

// adjustParallelLimit takes a number of objects and a proposed limit and
// figures out if it's reasonable (and adjusts it accordingly). This is only
// used for daemon startup, which does a lot of parallel loading of containers
// (and if we exceed RLIMIT_NOFILE then we're in trouble).
func adjustParallelLimit(n int, limit int) int {
	// Rule-of-thumb overhead factor (how many files will each goroutine open
	// simultaneously). Yes, this is ugly but to be frank this whole thing is
	// ugly.
	const overhead = 2

	// On Linux, we need to ensure that parallelStartupJobs doesn't cause us to
	// exceed RLIMIT_NOFILE. If parallelStartupJobs is too large, we reduce it
	// and give a warning (since in theory the user should increase their
	// ulimits to the largest possible value for dockerd).
	var rlim unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_NOFILE, &rlim); err != nil {
		logrus.Warnf("Couldn't find dockerd's RLIMIT_NOFILE to double-check startup parallelism factor: %v", err)
		return limit
	}
	softRlimit := int(rlim.Cur)

	// Much fewer containers than RLIMIT_NOFILE. No need to adjust anything.
	if softRlimit > overhead*n {
		return limit
	}

	// RLIMIT_NOFILE big enough, no need to adjust anything.
	if softRlimit > overhead*limit {
		return limit
	}

	logrus.Warnf("Found dockerd's open file ulimit (%v) is far too small -- consider increasing it significantly (at least %v)", softRlimit, overhead*limit)
	return softRlimit / overhead
}

// adaptContainerSettings is called during container creation to modify any
// settings necessary in the HostConfig structure.
func (daemon *Daemon) adaptContainerSettings(hostConfig *containertypes.HostConfig, adjustCPUShares bool) error {
	if adjustCPUShares && hostConfig.CPUShares > 0 {
		// Handle unsupported CPUShares
		if hostConfig.CPUShares < linuxMinCPUShares {
			logrus.Warnf("Changing requested CPUShares of %d to minimum allowed of %d", hostConfig.CPUShares, linuxMinCPUShares)
			hostConfig.CPUShares = linuxMinCPUShares
		} else if hostConfig.CPUShares > linuxMaxCPUShares {
			logrus.Warnf("Changing requested CPUShares of %d to maximum allowed of %d", hostConfig.CPUShares, linuxMaxCPUShares)
			hostConfig.CPUShares = linuxMaxCPUShares
		}
	}
	if hostConfig.Memory > 0 && hostConfig.MemorySwap == 0 {
		// By default, MemorySwap is set to twice the size of Memory.
		hostConfig.MemorySwap = hostConfig.Memory * 2
	}
	if hostConfig.ShmSize == 0 {
		hostConfig.ShmSize = config.DefaultShmSize
		if daemon.configStore != nil {
			hostConfig.ShmSize = int64(daemon.configStore.ShmSize)
		}
	}
	// Set default IPC mode, if unset for container
	if hostConfig.IpcMode.IsEmpty() {
		m := config.DefaultIpcMode
		if daemon.configStore != nil {
			m = containertypes.IpcMode(daemon.configStore.IpcMode)
		}
		hostConfig.IpcMode = m
	}

	// Set default cgroup namespace mode, if unset for container
	if hostConfig.CgroupnsMode.IsEmpty() {
		// for cgroup v2: unshare cgroupns even for privileged containers
		// https://github.com/containers/libpod/pull/4374#issuecomment-549776387
		if hostConfig.Privileged && cgroups.Mode() != cgroups.Unified {
			hostConfig.CgroupnsMode = containertypes.CgroupnsModeHost
		} else {
			m := containertypes.CgroupnsModeHost
			if cgroups.Mode() == cgroups.Unified {
				m = containertypes.CgroupnsModePrivate
			}
			if daemon.configStore != nil {
				m = containertypes.CgroupnsMode(daemon.configStore.CgroupNamespaceMode)
			}
			hostConfig.CgroupnsMode = m
		}
	}

	adaptSharedNamespaceContainer(daemon, hostConfig)

	var err error
	secOpts, err := daemon.generateSecurityOpt(hostConfig)
	if err != nil {
		return err
	}
	hostConfig.SecurityOpt = append(hostConfig.SecurityOpt, secOpts...)
	if hostConfig.OomKillDisable == nil {
		defaultOomKillDisable := false
		hostConfig.OomKillDisable = &defaultOomKillDisable
	}

	return nil
}

// adaptSharedNamespaceContainer replaces container name with its ID in hostConfig.
// To be more precisely, it modifies `container:name` to `container:ID` of PidMode, IpcMode
// and NetworkMode.
//
// When a container shares its namespace with another container, use ID can keep the namespace
// sharing connection between the two containers even the another container is renamed.
func adaptSharedNamespaceContainer(daemon containerGetter, hostConfig *containertypes.HostConfig) {
	containerPrefix := "container:"
	if hostConfig.PidMode.IsContainer() {
		pidContainer := hostConfig.PidMode.Container()
		// if there is any error returned here, we just ignore it and leave it to be
		// handled in the following logic
		if c, err := daemon.GetContainer(pidContainer); err == nil {
			hostConfig.PidMode = containertypes.PidMode(containerPrefix + c.ID)
		}
	}
	if hostConfig.IpcMode.IsContainer() {
		ipcContainer := hostConfig.IpcMode.Container()
		if c, err := daemon.GetContainer(ipcContainer); err == nil {
			hostConfig.IpcMode = containertypes.IpcMode(containerPrefix + c.ID)
		}
	}
	if hostConfig.NetworkMode.IsContainer() {
		netContainer := hostConfig.NetworkMode.ConnectedContainer()
		if c, err := daemon.GetContainer(netContainer); err == nil {
			hostConfig.NetworkMode = containertypes.NetworkMode(containerPrefix + c.ID)
		}
	}
}

// verifyPlatformContainerResources performs platform-specific validation of the container's resource-configuration
func verifyPlatformContainerResources(resources *containertypes.Resources, sysInfo *sysinfo.SysInfo, update bool) (warnings []string, err error) {
	fixMemorySwappiness(resources)

	// memory subsystem checks and adjustments
	if resources.Memory != 0 && resources.Memory < linuxMinMemory {
		return warnings, fmt.Errorf("Minimum memory limit allowed is 6MB")
	}
	if resources.Memory > 0 && !sysInfo.MemoryLimit {
		warnings = append(warnings, "Your kernel does not support memory limit capabilities or the cgroup is not mounted. Limitation discarded.")
		resources.Memory = 0
		resources.MemorySwap = -1
	}
	if resources.Memory > 0 && resources.MemorySwap != -1 && !sysInfo.SwapLimit {
		warnings = append(warnings, "Your kernel does not support swap limit capabilities or the cgroup is not mounted. Memory limited without swap.")
		resources.MemorySwap = -1
	}
	if resources.Memory > 0 && resources.MemorySwap > 0 && resources.MemorySwap < resources.Memory {
		return warnings, fmt.Errorf("Minimum memoryswap limit should be larger than memory limit, see usage")
	}
	if resources.Memory == 0 && resources.MemorySwap > 0 && !update {
		return warnings, fmt.Errorf("You should always set the Memory limit when using Memoryswap limit, see usage")
	}
	if resources.MemorySwappiness != nil && !sysInfo.MemorySwappiness {
		warnings = append(warnings, "Your kernel does not support memory swappiness capabilities or the cgroup is not mounted. Memory swappiness discarded.")
		resources.MemorySwappiness = nil
	}
	if resources.MemorySwappiness != nil {
		swappiness := *resources.MemorySwappiness
		if swappiness < 0 || swappiness > 100 {
			return warnings, fmt.Errorf("Invalid value: %v, valid memory swappiness range is 0-100", swappiness)
		}
	}
	if resources.MemoryReservation > 0 && !sysInfo.MemoryReservation {
		warnings = append(warnings, "Your kernel does not support memory soft limit capabilities or the cgroup is not mounted. Limitation discarded.")
		resources.MemoryReservation = 0
	}
	if resources.MemoryReservation > 0 && resources.MemoryReservation < linuxMinMemory {
		return warnings, fmt.Errorf("Minimum memory reservation allowed is 6MB")
	}
	if resources.Memory > 0 && resources.MemoryReservation > 0 && resources.Memory < resources.MemoryReservation {
		return warnings, fmt.Errorf("Minimum memory limit can not be less than memory reservation limit, see usage")
	}
	if resources.KernelMemory > 0 {
		// Kernel memory limit is not supported on cgroup v2.
		// Even on cgroup v1, kernel memory limit (`kmem.limit_in_bytes`) has been deprecated since kernel 5.4.
		// https://github.com/torvalds/linux/commit/0158115f702b0ba208ab0b5adf44cae99b3ebcc7
		if !sysInfo.KernelMemory {
			warnings = append(warnings, "Your kernel does not support kernel memory limit capabilities or the cgroup is not mounted. Limitation discarded.")
			resources.KernelMemory = 0
		}
		if resources.KernelMemory > 0 && resources.KernelMemory < linuxMinMemory {
			return warnings, fmt.Errorf("Minimum kernel memory limit allowed is 6MB")
		}
		if !kernel.CheckKernelVersion(4, 0, 0) {
			warnings = append(warnings, "You specified a kernel memory limit on a kernel older than 4.0. Kernel memory limits are experimental on older kernels, it won't work as expected and can cause your system to be unstable.")
		}
	}
	if resources.OomKillDisable != nil && !sysInfo.OomKillDisable {
		// only produce warnings if the setting wasn't to *disable* the OOM Kill; no point
		// warning the caller if they already wanted the feature to be off
		if *resources.OomKillDisable {
			warnings = append(warnings, "Your kernel does not support OomKillDisable. OomKillDisable discarded.")
		}
		resources.OomKillDisable = nil
	}
	if resources.OomKillDisable != nil && *resources.OomKillDisable && resources.Memory == 0 {
		warnings = append(warnings, "OOM killer is disabled for the container, but no memory limit is set, this can result in the system running out of resources.")
	}
	if resources.PidsLimit != nil && !sysInfo.PidsLimit {
		if *resources.PidsLimit > 0 {
			warnings = append(warnings, "Your kernel does not support PIDs limit capabilities or the cgroup is not mounted. PIDs limit discarded.")
		}
		resources.PidsLimit = nil
	}

	// cpu subsystem checks and adjustments
	if resources.NanoCPUs > 0 && resources.CPUPeriod > 0 {
		return warnings, fmt.Errorf("Conflicting options: Nano CPUs and CPU Period cannot both be set")
	}
	if resources.NanoCPUs > 0 && resources.CPUQuota > 0 {
		return warnings, fmt.Errorf("Conflicting options: Nano CPUs and CPU Quota cannot both be set")
	}
	if resources.NanoCPUs > 0 && !sysInfo.CPUCfs {
		return warnings, fmt.Errorf("NanoCPUs can not be set, as your kernel does not support CPU CFS scheduler or the cgroup is not mounted")
	}
	// The highest precision we could get on Linux is 0.001, by setting
	//   cpu.cfs_period_us=1000ms
	//   cpu.cfs_quota=1ms
	// See the following link for details:
	// https://www.kernel.org/doc/Documentation/scheduler/sched-bwc.txt
	// Here we don't set the lower limit and it is up to the underlying platform (e.g., Linux) to return an error.
	// The error message is 0.01 so that this is consistent with Windows
	if resources.NanoCPUs < 0 || resources.NanoCPUs > int64(sysinfo.NumCPU())*1e9 {
		return warnings, fmt.Errorf("Range of CPUs is from 0.01 to %d.00, as there are only %d CPUs available", sysinfo.NumCPU(), sysinfo.NumCPU())
	}

	if resources.CPUShares > 0 && !sysInfo.CPUShares {
		warnings = append(warnings, "Your kernel does not support CPU shares or the cgroup is not mounted. Shares discarded.")
		resources.CPUShares = 0
	}
	if (resources.CPUPeriod != 0 || resources.CPUQuota != 0) && !sysInfo.CPUCfs {
		warnings = append(warnings, "Your kernel does not support CPU CFS scheduler. CPU period/quota discarded.")
		resources.CPUPeriod = 0
		resources.CPUQuota = 0
	}
	if resources.CPUPeriod != 0 && (resources.CPUPeriod < 1000 || resources.CPUPeriod > 1000000) {
		return warnings, fmt.Errorf("CPU cfs period can not be less than 1ms (i.e. 1000) or larger than 1s (i.e. 1000000)")
	}
	if resources.CPUQuota > 0 && resources.CPUQuota < 1000 {
		return warnings, fmt.Errorf("CPU cfs quota can not be less than 1ms (i.e. 1000)")
	}
	if resources.CPUPercent > 0 {
		warnings = append(warnings, fmt.Sprintf("%s does not support CPU percent. Percent discarded.", runtime.GOOS))
		resources.CPUPercent = 0
	}

	// cpuset subsystem checks and adjustments
	if (resources.CpusetCpus != "" || resources.CpusetMems != "") && !sysInfo.Cpuset {
		warnings = append(warnings, "Your kernel does not support cpuset or the cgroup is not mounted. Cpuset discarded.")
		resources.CpusetCpus = ""
		resources.CpusetMems = ""
	}
	cpusAvailable, err := sysInfo.IsCpusetCpusAvailable(resources.CpusetCpus)
	if err != nil {
		return warnings, errors.Wrapf(err, "Invalid value %s for cpuset cpus", resources.CpusetCpus)
	}
	if !cpusAvailable {
		return warnings, fmt.Errorf("Requested CPUs are not available - requested %s, available: %s", resources.CpusetCpus, sysInfo.Cpus)
	}
	memsAvailable, err := sysInfo.IsCpusetMemsAvailable(resources.CpusetMems)
	if err != nil {
		return warnings, errors.Wrapf(err, "Invalid value %s for cpuset mems", resources.CpusetMems)
	}
	if !memsAvailable {
		return warnings, fmt.Errorf("Requested memory nodes are not available - requested %s, available: %s", resources.CpusetMems, sysInfo.Mems)
	}

	// blkio subsystem checks and adjustments
	if resources.BlkioWeight > 0 && !sysInfo.BlkioWeight {
		warnings = append(warnings, "Your kernel does not support Block I/O weight or the cgroup is not mounted. Weight discarded.")
		resources.BlkioWeight = 0
	}
	if resources.BlkioWeight > 0 && (resources.BlkioWeight < 10 || resources.BlkioWeight > 1000) {
		return warnings, fmt.Errorf("Range of blkio weight is from 10 to 1000")
	}
	if resources.IOMaximumBandwidth != 0 || resources.IOMaximumIOps != 0 {
		return warnings, fmt.Errorf("Invalid QoS settings: %s does not support Maximum IO Bandwidth or Maximum IO IOps", runtime.GOOS)
	}
	if len(resources.BlkioWeightDevice) > 0 && !sysInfo.BlkioWeightDevice {
		warnings = append(warnings, "Your kernel does not support Block I/O weight_device or the cgroup is not mounted. Weight-device discarded.")
		resources.BlkioWeightDevice = []*pblkiodev.WeightDevice{}
	}
	if len(resources.BlkioDeviceReadBps) > 0 && !sysInfo.BlkioReadBpsDevice {
		warnings = append(warnings, "Your kernel does not support BPS Block I/O read limit or the cgroup is not mounted. Block I/O BPS read limit discarded.")
		resources.BlkioDeviceReadBps = []*pblkiodev.ThrottleDevice{}
	}
	if len(resources.BlkioDeviceWriteBps) > 0 && !sysInfo.BlkioWriteBpsDevice {
		warnings = append(warnings, "Your kernel does not support BPS Block I/O write limit or the cgroup is not mounted. Block I/O BPS write limit discarded.")
		resources.BlkioDeviceWriteBps = []*pblkiodev.ThrottleDevice{}
	}
	if len(resources.BlkioDeviceReadIOps) > 0 && !sysInfo.BlkioReadIOpsDevice {
		warnings = append(warnings, "Your kernel does not support IOPS Block read limit or the cgroup is not mounted. Block I/O IOPS read limit discarded.")
		resources.BlkioDeviceReadIOps = []*pblkiodev.ThrottleDevice{}
	}
	if len(resources.BlkioDeviceWriteIOps) > 0 && !sysInfo.BlkioWriteIOpsDevice {
		warnings = append(warnings, "Your kernel does not support IOPS Block write limit or the cgroup is not mounted. Block I/O IOPS write limit discarded.")
		resources.BlkioDeviceWriteIOps = []*pblkiodev.ThrottleDevice{}
	}

	return warnings, nil
}

func (daemon *Daemon) getCgroupDriver() string {
	if UsingSystemd(daemon.configStore) {
		return cgroupSystemdDriver
	}
	if daemon.Rootless() {
		return cgroupNoneDriver
	}
	return cgroupFsDriver
}

// getCD gets the raw value of the native.cgroupdriver option, if set.
func getCD(config *config.Config) string {
	for _, option := range config.ExecOptions {
		key, val, err := parsers.ParseKeyValueOpt(option)
		if err != nil || !strings.EqualFold(key, "native.cgroupdriver") {
			continue
		}
		return val
	}
	return ""
}

// verifyCgroupDriver validates native.cgroupdriver
func verifyCgroupDriver(config *config.Config) error {
	cd := getCD(config)
	if cd == "" || cd == cgroupFsDriver || cd == cgroupSystemdDriver {
		return nil
	}
	if cd == cgroupNoneDriver {
		return fmt.Errorf("native.cgroupdriver option %s is internally used and cannot be specified manually", cd)
	}
	return fmt.Errorf("native.cgroupdriver option %s not supported", cd)
}

// UsingSystemd returns true if cli option includes native.cgroupdriver=systemd
func UsingSystemd(config *config.Config) bool {
	cd := getCD(config)

	if cd == cgroupSystemdDriver {
		return true
	}
	// On cgroup v2 hosts, default to systemd driver
	if cd == "" && cgroups.Mode() == cgroups.Unified && isRunningSystemd() {
		return true
	}
	return false
}

var (
	runningSystemd bool
	detectSystemd  sync.Once
)

// isRunningSystemd checks whether the host was booted with systemd as its init
// system. This functions similarly to systemd's `sd_booted(3)`: internally, it
// checks whether /run/systemd/system/ exists and is a directory.
// http://www.freedesktop.org/software/systemd/man/sd_booted.html
//
// NOTE: This function comes from package github.com/coreos/go-systemd/util
// It was borrowed here to avoid a dependency on cgo.
func isRunningSystemd() bool {
	detectSystemd.Do(func() {
		fi, err := os.Lstat("/run/systemd/system")
		if err != nil {
			return
		}
		runningSystemd = fi.IsDir()
	})
	return runningSystemd
}

// verifyPlatformContainerSettings performs platform-specific validation of the
// hostconfig and config structures.
func verifyPlatformContainerSettings(daemon *Daemon, hostConfig *containertypes.HostConfig, update bool) (warnings []string, err error) {
	if hostConfig == nil {
		return nil, nil
	}
	sysInfo := daemon.RawSysInfo()

	w, err := verifyPlatformContainerResources(&hostConfig.Resources, sysInfo, update)

	// no matter err is nil or not, w could have data in itself.
	warnings = append(warnings, w...)

	if err != nil {
		return warnings, err
	}

	if !hostConfig.IpcMode.Valid() {
		return warnings, errors.Errorf("invalid IPC mode: %v", hostConfig.IpcMode)
	}
	if !hostConfig.PidMode.Valid() {
		return warnings, errors.Errorf("invalid PID mode: %v", hostConfig.PidMode)
	}
	if hostConfig.ShmSize < 0 {
		return warnings, fmt.Errorf("SHM size can not be less than 0")
	}
	if !hostConfig.UTSMode.Valid() {
		return warnings, errors.Errorf("invalid UTS mode: %v", hostConfig.UTSMode)
	}

	if hostConfig.OomScoreAdj < -1000 || hostConfig.OomScoreAdj > 1000 {
		return warnings, fmt.Errorf("Invalid value %d, range for oom score adj is [-1000, 1000]", hostConfig.OomScoreAdj)
	}

	// ip-forwarding does not affect container with '--net=host' (or '--net=none')
	if sysInfo.IPv4ForwardingDisabled && !(hostConfig.NetworkMode.IsHost() || hostConfig.NetworkMode.IsNone()) {
		warnings = append(warnings, "IPv4 forwarding is disabled. Networking will not work.")
	}
	if hostConfig.NetworkMode.IsHost() && len(hostConfig.PortBindings) > 0 {
		warnings = append(warnings, "Published ports are discarded when using host network mode")
	}

	// check for various conflicting options with user namespaces
	if daemon.configStore.RemappedRoot != "" && hostConfig.UsernsMode.IsPrivate() {
		if hostConfig.Privileged {
			return warnings, fmt.Errorf("privileged mode is incompatible with user namespaces.  You must run the container in the host namespace when running privileged mode")
		}
		if hostConfig.NetworkMode.IsHost() && !hostConfig.UsernsMode.IsHost() {
			return warnings, fmt.Errorf("cannot share the host's network namespace when user namespaces are enabled")
		}
		if hostConfig.PidMode.IsHost() && !hostConfig.UsernsMode.IsHost() {
			return warnings, fmt.Errorf("cannot share the host PID namespace when user namespaces are enabled")
		}
	}
	if hostConfig.CgroupParent != "" && UsingSystemd(daemon.configStore) {
		// CgroupParent for systemd cgroup should be named as "xxx.slice"
		if len(hostConfig.CgroupParent) <= 6 || !strings.HasSuffix(hostConfig.CgroupParent, ".slice") {
			return warnings, fmt.Errorf("cgroup-parent for systemd cgroup should be a valid slice named as \"xxx.slice\"")
		}
	}
	if hostConfig.Runtime == "" {
		hostConfig.Runtime = daemon.configStore.GetDefaultRuntimeName()
	}

	if _, _, err := daemon.getRuntime(hostConfig.Runtime); err != nil {
		return warnings, err
	}

	parser := volumemounts.NewParser()
	for dest := range hostConfig.Tmpfs {
		if err := parser.ValidateTmpfsMountDestination(dest); err != nil {
			return warnings, err
		}
	}

	if !hostConfig.CgroupnsMode.Valid() {
		return warnings, fmt.Errorf("invalid cgroup namespace mode: %v", hostConfig.CgroupnsMode)
	}
	if hostConfig.CgroupnsMode.IsPrivate() {
		if !sysInfo.CgroupNamespaces {
			warnings = append(warnings, "Your kernel does not support cgroup namespaces.  Cgroup namespace setting discarded.")
		}
	}

	return warnings, nil
}

// verifyDaemonSettings performs validation of daemon config struct
func verifyDaemonSettings(conf *config.Config) error {
	if conf.ContainerdNamespace == conf.ContainerdPluginNamespace {
		return errors.New("containers namespace and plugins namespace cannot be the same")
	}
	// Check for mutually incompatible config options
	if conf.BridgeConfig.Iface != "" && conf.BridgeConfig.IP != "" {
		return fmt.Errorf("You specified -b & --bip, mutually exclusive options. Please specify only one")
	}
	if !conf.BridgeConfig.EnableIPTables && !conf.BridgeConfig.InterContainerCommunication {
		return fmt.Errorf("You specified --iptables=false with --icc=false. ICC=false uses iptables to function. Please set --icc or --iptables to true")
	}
	if conf.BridgeConfig.EnableIP6Tables && !conf.Experimental {
		return fmt.Errorf("ip6tables rules are only available if experimental features are enabled")
	}
	if !conf.BridgeConfig.EnableIPTables && conf.BridgeConfig.EnableIPMasq {
		conf.BridgeConfig.EnableIPMasq = false
	}
	if err := verifyCgroupDriver(conf); err != nil {
		return err
	}
	if conf.CgroupParent != "" && UsingSystemd(conf) {
		if len(conf.CgroupParent) <= 6 || !strings.HasSuffix(conf.CgroupParent, ".slice") {
			return fmt.Errorf("cgroup-parent for systemd cgroup should be a valid slice named as \"xxx.slice\"")
		}
	}

	if conf.Rootless && UsingSystemd(conf) && cgroups.Mode() != cgroups.Unified {
		return fmt.Errorf("exec-opt native.cgroupdriver=systemd requires cgroup v2 for rootless mode")
	}

	configureRuntimes(conf)
	if rtName := conf.GetDefaultRuntimeName(); rtName != "" {
		if conf.GetRuntime(rtName) == nil {
			if !config.IsPermissibleC8dRuntimeName(rtName) {
				return fmt.Errorf("specified default runtime '%s' does not exist", rtName)
			}
		}
	}
	return nil
}

// checkSystem validates platform-specific requirements
func checkSystem() error {
	return nil
}

// configureMaxThreads sets the Go runtime max threads threshold
// which is 90% of the kernel setting from /proc/sys/kernel/threads-max
func configureMaxThreads(config *config.Config) error {
	mt, err := os.ReadFile("/proc/sys/kernel/threads-max")
	if err != nil {
		return err
	}
	mtint, err := strconv.Atoi(strings.TrimSpace(string(mt)))
	if err != nil {
		return err
	}
	maxThreads := (mtint / 100) * 90
	debug.SetMaxThreads(maxThreads)
	logrus.Debugf("Golang's threads limit set to %d", maxThreads)
	return nil
}

func overlaySupportsSelinux() (bool, error) {
	f, err := os.Open("/proc/kallsyms")
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		if strings.HasSuffix(s.Text(), " security_inode_copy_up") {
			return true, nil
		}
	}

	return false, s.Err()
}

// configureKernelSecuritySupport configures and validates security support for the kernel
func configureKernelSecuritySupport(config *config.Config, driverName string) error {
	if config.EnableSelinuxSupport {
		if !selinux.GetEnabled() {
			logrus.Warn("Docker could not enable SELinux on the host system")
			return nil
		}

		if driverName == "overlay2" || driverName == "overlayfs" {
			// If driver is overlay2, make sure kernel
			// supports selinux with overlay.
			supported, err := overlaySupportsSelinux()
			if err != nil {
				return err
			}

			if !supported {
				logrus.Warnf("SELinux is not supported with the %v graph driver on this kernel", driverName)
			}
		}
	} else {
		selinux.SetDisabled()
	}
	return nil
}

// initNetworkController initializes the libnetwork controller and configures
// network settings. If there's active sandboxes, configuration changes will not
// take effect.
func (daemon *Daemon) initNetworkController(activeSandboxes map[string]interface{}) error {
	netOptions, err := daemon.networkOptions(daemon.PluginStore, activeSandboxes)
	if err != nil {
		return err
	}

	daemon.netController, err = libnetwork.New(netOptions...)
	if err != nil {
		return fmt.Errorf("error obtaining controller instance: %v", err)
	}

	if len(activeSandboxes) > 0 {
		logrus.Info("there are running containers, updated network configuration will not take affect")
	} else if err := configureNetworking(daemon.netController, daemon.configStore); err != nil {
		return err
	}

	// Set HostGatewayIP to the default bridge's IP if it is empty
	setHostGatewayIP(daemon.netController, daemon.configStore)
	return nil
}

func configureNetworking(controller *libnetwork.Controller, conf *config.Config) error {
	// Initialize default network on "null"
	if n, _ := controller.NetworkByName("none"); n == nil {
		if _, err := controller.NewNetwork("null", "none", "", libnetwork.NetworkOptionPersist(true)); err != nil {
			return errors.Wrap(err, `error creating default "null" network`)
		}
	}

	// Initialize default network on "host"
	if n, _ := controller.NetworkByName("host"); n == nil {
		if _, err := controller.NewNetwork("host", "host", "", libnetwork.NetworkOptionPersist(true)); err != nil {
			return errors.Wrap(err, `error creating default "host" network`)
		}
	}

	// Clear stale bridge network
	if n, err := controller.NetworkByName("bridge"); err == nil {
		if err = n.Delete(); err != nil {
			return errors.Wrap(err, `could not delete the default "bridge"" network`)
		}
		if len(conf.NetworkConfig.DefaultAddressPools.Value()) > 0 && !conf.LiveRestoreEnabled {
			removeDefaultBridgeInterface()
		}
	}

	if !conf.DisableBridge {
		// Initialize default driver "bridge"
		if err := initBridgeDriver(controller, conf); err != nil {
			return err
		}
	} else {
		removeDefaultBridgeInterface()
	}

	return nil
}

// setHostGatewayIP sets cfg.HostGatewayIP to the default bridge's IP if it is empty.
func setHostGatewayIP(controller *libnetwork.Controller, config *config.Config) {
	if config.HostGatewayIP != nil {
		return
	}
	if n, err := controller.NetworkByName("bridge"); err == nil {
		v4Info, v6Info := n.Info().IpamInfo()
		var gateway net.IP
		if len(v4Info) > 0 {
			gateway = v4Info[0].Gateway.IP
		} else if len(v6Info) > 0 {
			gateway = v6Info[0].Gateway.IP
		}
		config.HostGatewayIP = gateway
	}
}

func driverOptions(config *config.Config) nwconfig.Option {
	return nwconfig.OptionDriverConfig("bridge", options.Generic{
		netlabel.GenericData: options.Generic{
			"EnableIPForwarding":  config.BridgeConfig.EnableIPForward,
			"EnableIPTables":      config.BridgeConfig.EnableIPTables,
			"EnableIP6Tables":     config.BridgeConfig.EnableIP6Tables,
			"EnableUserlandProxy": config.BridgeConfig.EnableUserlandProxy,
			"UserlandProxyPath":   config.BridgeConfig.UserlandProxyPath,
		},
	})
}

func initBridgeDriver(controller *libnetwork.Controller, config *config.Config) error {
	bridgeName := bridge.DefaultBridgeName
	if config.BridgeConfig.Iface != "" {
		bridgeName = config.BridgeConfig.Iface
	}
	netOption := map[string]string{
		bridge.BridgeName:         bridgeName,
		bridge.DefaultBridge:      strconv.FormatBool(true),
		netlabel.DriverMTU:        strconv.Itoa(config.Mtu),
		bridge.EnableIPMasquerade: strconv.FormatBool(config.BridgeConfig.EnableIPMasq),
		bridge.EnableICC:          strconv.FormatBool(config.BridgeConfig.InterContainerCommunication),
	}

	// --ip processing
	if config.BridgeConfig.DefaultIP != nil {
		netOption[bridge.DefaultBindingIP] = config.BridgeConfig.DefaultIP.String()
	}

	ipamV4Conf := &libnetwork.IpamConf{AuxAddresses: make(map[string]string)}

	// By default, libnetwork will request an arbitrary available address
	// pool for the network from the configured IPAM allocator.
	// Configure it to use the IPv4 network ranges of the existing bridge
	// interface if one exists with IPv4 addresses assigned to it.

	nwList, nw6List, err := ifaceAddrs(bridgeName)
	if err != nil {
		return errors.Wrap(err, "list bridge addresses failed")
	}

	if len(nwList) > 0 {
		nw := nwList[0]
		if len(nwList) > 1 && config.BridgeConfig.FixedCIDR != "" {
			_, fCIDR, err := net.ParseCIDR(config.BridgeConfig.FixedCIDR)
			if err != nil {
				return errors.Wrap(err, "parse CIDR failed")
			}
			// Iterate through in case there are multiple addresses for the bridge
			for _, entry := range nwList {
				if fCIDR.Contains(entry.IP) {
					nw = entry
					break
				}
			}
		}

		ipamV4Conf.PreferredPool = lntypes.GetIPNetCanonical(nw).String()
		hip, _ := lntypes.GetHostPartIP(nw.IP, nw.Mask)
		if hip.IsGlobalUnicast() {
			ipamV4Conf.Gateway = nw.IP.String()
		}
	}

	if config.BridgeConfig.IP != "" {
		ip, ipNet, err := net.ParseCIDR(config.BridgeConfig.IP)
		if err != nil {
			return err
		}
		ipamV4Conf.PreferredPool = ipNet.String()
		ipamV4Conf.Gateway = ip.String()
	} else if bridgeName == bridge.DefaultBridgeName && ipamV4Conf.PreferredPool != "" {
		logrus.Infof("Default bridge (%s) is assigned with an IP address %s. Daemon option --bip can be used to set a preferred IP address", bridgeName, ipamV4Conf.PreferredPool)
	}

	if config.BridgeConfig.FixedCIDR != "" {
		_, fCIDR, err := net.ParseCIDR(config.BridgeConfig.FixedCIDR)
		if err != nil {
			return err
		}

		ipamV4Conf.SubPool = fCIDR.String()
		if ipamV4Conf.PreferredPool == "" {
			ipamV4Conf.PreferredPool = fCIDR.String()
		}
	}

	if config.BridgeConfig.DefaultGatewayIPv4 != nil {
		ipamV4Conf.AuxAddresses["DefaultGatewayIPv4"] = config.BridgeConfig.DefaultGatewayIPv4.String()
	}

	var (
		deferIPv6Alloc bool
		ipamV6Conf     *libnetwork.IpamConf
	)

	if config.BridgeConfig.EnableIPv6 && config.BridgeConfig.FixedCIDRv6 == "" {
		return errdefs.InvalidParameter(errors.New("IPv6 is enabled for the default bridge, but no subnet is configured. Specify an IPv6 subnet using --fixed-cidr-v6"))
	} else if config.BridgeConfig.FixedCIDRv6 != "" {
		_, fCIDRv6, err := net.ParseCIDR(config.BridgeConfig.FixedCIDRv6)
		if err != nil {
			return err
		}

		// In case user has specified the daemon flag --fixed-cidr-v6 and the passed network has
		// at least 48 host bits, we need to guarantee the current behavior where the containers'
		// IPv6 addresses will be constructed based on the containers' interface MAC address.
		// We do so by telling libnetwork to defer the IPv6 address allocation for the endpoints
		// on this network until after the driver has created the endpoint and returned the
		// constructed address. Libnetwork will then reserve this address with the ipam driver.
		ones, _ := fCIDRv6.Mask.Size()
		deferIPv6Alloc = ones <= 80

		ipamV6Conf = &libnetwork.IpamConf{
			AuxAddresses:  make(map[string]string),
			PreferredPool: fCIDRv6.String(),
		}

		// In case the --fixed-cidr-v6 is specified and the current docker0 bridge IPv6
		// address belongs to the same network, we need to inform libnetwork about it, so
		// that it can be reserved with IPAM and it will not be given away to somebody else
		for _, nw6 := range nw6List {
			if fCIDRv6.Contains(nw6.IP) {
				ipamV6Conf.Gateway = nw6.IP.String()
				break
			}
		}
	}

	if config.BridgeConfig.DefaultGatewayIPv6 != nil {
		if ipamV6Conf == nil {
			ipamV6Conf = &libnetwork.IpamConf{AuxAddresses: make(map[string]string)}
		}
		ipamV6Conf.AuxAddresses["DefaultGatewayIPv6"] = config.BridgeConfig.DefaultGatewayIPv6.String()
	}

	v4Conf := []*libnetwork.IpamConf{ipamV4Conf}
	v6Conf := []*libnetwork.IpamConf{}
	if ipamV6Conf != nil {
		v6Conf = append(v6Conf, ipamV6Conf)
	}
	// Initialize default network on "bridge" with the same name
	_, err = controller.NewNetwork("bridge", "bridge", "",
		libnetwork.NetworkOptionEnableIPv6(config.BridgeConfig.EnableIPv6),
		libnetwork.NetworkOptionDriverOpts(netOption),
		libnetwork.NetworkOptionIpam("default", "", v4Conf, v6Conf, nil),
		libnetwork.NetworkOptionDeferIPv6Alloc(deferIPv6Alloc))
	if err != nil {
		return fmt.Errorf("Error creating default \"bridge\" network: %v", err)
	}
	return nil
}

// Remove default bridge interface if present (--bridge=none use case)
func removeDefaultBridgeInterface() {
	if lnk, err := netlink.LinkByName(bridge.DefaultBridgeName); err == nil {
		if err := netlink.LinkDel(lnk); err != nil {
			logrus.Warnf("Failed to remove bridge interface (%s): %v", bridge.DefaultBridgeName, err)
		}
	}
}

func setupInitLayer(idMapping idtools.IdentityMapping) func(string) error {
	return func(initPath string) error {
		return initlayer.Setup(initPath, idMapping.RootPair())
	}
}

// Parse the remapped root (user namespace) option, which can be one of:
//
// - username            - valid username from /etc/passwd
// - username:groupname  - valid username; valid groupname from /etc/group
// - uid                 - 32-bit unsigned int valid Linux UID value
// - uid:gid             - uid value; 32-bit unsigned int Linux GID value
//
// If no groupname is specified, and a username is specified, an attempt
// will be made to lookup a gid for that username as a groupname
//
// If names are used, they are verified to exist in passwd/group
func parseRemappedRoot(usergrp string) (string, string, error) {
	var (
		userID, groupID     int
		username, groupname string
	)

	idparts := strings.Split(usergrp, ":")
	if len(idparts) > 2 {
		return "", "", fmt.Errorf("Invalid user/group specification in --userns-remap: %q", usergrp)
	}

	if uid, err := strconv.ParseInt(idparts[0], 10, 32); err == nil {
		// must be a uid; take it as valid
		userID = int(uid)
		luser, err := idtools.LookupUID(userID)
		if err != nil {
			return "", "", fmt.Errorf("Uid %d has no entry in /etc/passwd: %v", userID, err)
		}
		username = luser.Name
		if len(idparts) == 1 {
			// if the uid was numeric and no gid was specified, take the uid as the gid
			groupID = userID
			lgrp, err := idtools.LookupGID(groupID)
			if err != nil {
				return "", "", fmt.Errorf("Gid %d has no entry in /etc/group: %v", groupID, err)
			}
			groupname = lgrp.Name
		}
	} else {
		lookupName := idparts[0]
		// special case: if the user specified "default", they want Docker to create or
		// use (after creation) the "dockremap" user/group for root remapping
		if lookupName == defaultIDSpecifier {
			lookupName = defaultRemappedID
		}
		luser, err := idtools.LookupUser(lookupName)
		if err != nil && idparts[0] != defaultIDSpecifier {
			// error if the name requested isn't the special "dockremap" ID
			return "", "", fmt.Errorf("Error during uid lookup for %q: %v", lookupName, err)
		} else if err != nil {
			// special case-- if the username == "default", then we have been asked
			// to create a new entry pair in /etc/{passwd,group} for which the /etc/sub{uid,gid}
			// ranges will be used for the user and group mappings in user namespaced containers
			_, _, err := idtools.AddNamespaceRangesUser(defaultRemappedID)
			if err == nil {
				return defaultRemappedID, defaultRemappedID, nil
			}
			return "", "", fmt.Errorf("Error during %q user creation: %v", defaultRemappedID, err)
		}
		username = luser.Name
		if len(idparts) == 1 {
			// we only have a string username, and no group specified; look up gid from username as group
			group, err := idtools.LookupGroup(lookupName)
			if err != nil {
				return "", "", fmt.Errorf("Error during gid lookup for %q: %v", lookupName, err)
			}
			groupname = group.Name
		}
	}

	if len(idparts) == 2 {
		// groupname or gid is separately specified and must be resolved
		// to an unsigned 32-bit gid
		if gid, err := strconv.ParseInt(idparts[1], 10, 32); err == nil {
			// must be a gid, take it as valid
			groupID = int(gid)
			lgrp, err := idtools.LookupGID(groupID)
			if err != nil {
				return "", "", fmt.Errorf("Gid %d has no entry in /etc/passwd: %v", groupID, err)
			}
			groupname = lgrp.Name
		} else {
			// not a number; attempt a lookup
			if _, err := idtools.LookupGroup(idparts[1]); err != nil {
				return "", "", fmt.Errorf("Error during groupname lookup for %q: %v", idparts[1], err)
			}
			groupname = idparts[1]
		}
	}
	return username, groupname, nil
}

func setupRemappedRoot(config *config.Config) (idtools.IdentityMapping, error) {
	if runtime.GOOS != "linux" && config.RemappedRoot != "" {
		return idtools.IdentityMapping{}, fmt.Errorf("User namespaces are only supported on Linux")
	}

	// if the daemon was started with remapped root option, parse
	// the config option to the int uid,gid values
	if config.RemappedRoot != "" {
		username, groupname, err := parseRemappedRoot(config.RemappedRoot)
		if err != nil {
			return idtools.IdentityMapping{}, err
		}
		if username == "root" {
			// Cannot setup user namespaces with a 1-to-1 mapping; "--root=0:0" is a no-op
			// effectively
			logrus.Warn("User namespaces: root cannot be remapped with itself; user namespaces are OFF")
			return idtools.IdentityMapping{}, nil
		}
		logrus.Infof("User namespaces: ID ranges will be mapped to subuid/subgid ranges of: %s", username)
		// update remapped root setting now that we have resolved them to actual names
		config.RemappedRoot = fmt.Sprintf("%s:%s", username, groupname)

		mappings, err := idtools.LoadIdentityMapping(username)
		if err != nil {
			return idtools.IdentityMapping{}, errors.Wrap(err, "Can't create ID mappings")
		}
		return mappings, nil
	}
	return idtools.IdentityMapping{}, nil
}

func setupDaemonRoot(config *config.Config, rootDir string, remappedRoot idtools.Identity) error {
	config.Root = rootDir
	// the docker root metadata directory needs to have execute permissions for all users (g+x,o+x)
	// so that syscalls executing as non-root, operating on subdirectories of the graph root
	// (e.g. mounted layers of a container) can traverse this path.
	// The user namespace support will create subdirectories for the remapped root host uid:gid
	// pair owned by that same uid:gid pair for proper write access to those needed metadata and
	// layer content subtrees.
	if _, err := os.Stat(rootDir); err == nil {
		// root current exists; verify the access bits are correct by setting them
		if err = os.Chmod(rootDir, 0711); err != nil {
			return err
		}
	} else if os.IsNotExist(err) {
		// no root exists yet, create it 0711 with root:root ownership
		if err := os.MkdirAll(rootDir, 0711); err != nil {
			return err
		}
	}

	id := idtools.Identity{UID: idtools.CurrentIdentity().UID, GID: remappedRoot.GID}
	// First make sure the current root dir has the correct perms.
	if err := idtools.MkdirAllAndChown(config.Root, 0710, id); err != nil {
		return errors.Wrapf(err, "could not create or set daemon root permissions: %s", config.Root)
	}

	// if user namespaces are enabled we will create a subtree underneath the specified root
	// with any/all specified remapped root uid/gid options on the daemon creating
	// a new subdirectory with ownership set to the remapped uid/gid (so as to allow
	// `chdir()` to work for containers namespaced to that uid/gid)
	if config.RemappedRoot != "" {
		config.Root = filepath.Join(rootDir, fmt.Sprintf("%d.%d", remappedRoot.UID, remappedRoot.GID))
		logrus.Debugf("Creating user namespaced daemon root: %s", config.Root)
		// Create the root directory if it doesn't exist
		if err := idtools.MkdirAllAndChown(config.Root, 0710, id); err != nil {
			return fmt.Errorf("Cannot create daemon root: %s: %v", config.Root, err)
		}
		// we also need to verify that any pre-existing directories in the path to
		// the graphroot won't block access to remapped root--if any pre-existing directory
		// has strict permissions that don't allow "x", container start will fail, so
		// better to warn and fail now
		dirPath := config.Root
		for {
			dirPath = filepath.Dir(dirPath)
			if dirPath == "/" {
				break
			}
			if !canAccess(dirPath, remappedRoot) {
				return fmt.Errorf("a subdirectory in your graphroot path (%s) restricts access to the remapped root uid/gid; please fix by allowing 'o+x' permissions on existing directories", config.Root)
			}
		}
	}

	if err := setupDaemonRootPropagation(config); err != nil {
		logrus.WithError(err).WithField("dir", config.Root).Warn("Error while setting daemon root propagation, this is not generally critical but may cause some functionality to not work or fallback to less desirable behavior")
	}
	return nil
}

// canAccess takes a valid (existing) directory and a uid, gid pair and determines
// if that uid, gid pair has access (execute bit) to the directory.
//
// Note: this is a very rudimentary check, and may not produce accurate results,
// so should not be used for anything other than the current use, see:
// https://github.com/moby/moby/issues/43724
func canAccess(path string, pair idtools.Identity) bool {
	statInfo, err := os.Stat(path)
	if err != nil {
		return false
	}
	perms := statInfo.Mode().Perm()
	if perms&0o001 == 0o001 {
		// world access
		return true
	}
	ssi := statInfo.Sys().(*syscall.Stat_t)
	if ssi.Uid == uint32(pair.UID) && (perms&0o100 == 0o100) {
		// owner access.
		return true
	}
	if ssi.Gid == uint32(pair.GID) && (perms&0o010 == 0o010) {
		// group access.
		return true
	}
	return false
}

func setupDaemonRootPropagation(cfg *config.Config) error {
	rootParentMount, mountOptions, err := getSourceMount(cfg.Root)
	if err != nil {
		return errors.Wrap(err, "error getting daemon root's parent mount")
	}

	var cleanupOldFile bool
	cleanupFile := getUnmountOnShutdownPath(cfg)
	defer func() {
		if !cleanupOldFile {
			return
		}
		if err := os.Remove(cleanupFile); err != nil && !os.IsNotExist(err) {
			logrus.WithError(err).WithField("file", cleanupFile).Warn("could not clean up old root propagation unmount file")
		}
	}()

	if hasMountInfoOption(mountOptions, sharedPropagationOption, slavePropagationOption) {
		cleanupOldFile = true
		return nil
	}

	if err := mount.MakeShared(cfg.Root); err != nil {
		return errors.Wrap(err, "could not setup daemon root propagation to shared")
	}

	// check the case where this may have already been a mount to itself.
	// If so then the daemon only performed a remount and should not try to unmount this later.
	if rootParentMount == cfg.Root {
		cleanupOldFile = true
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(cleanupFile), 0700); err != nil {
		return errors.Wrap(err, "error creating dir to store mount cleanup file")
	}

	if err := os.WriteFile(cleanupFile, nil, 0600); err != nil {
		return errors.Wrap(err, "error writing file to signal mount cleanup on shutdown")
	}
	return nil
}

// getUnmountOnShutdownPath generates the path to used when writing the file that signals to the daemon that on shutdown
// the daemon root should be unmounted.
func getUnmountOnShutdownPath(config *config.Config) string {
	return filepath.Join(config.ExecRoot, "unmount-on-shutdown")
}

// registerLinks registers network links between container and other containers
// with the daemon using the specification in hostConfig.
func (daemon *Daemon) registerLinks(container *container.Container, hostConfig *containertypes.HostConfig) error {
	if hostConfig == nil || hostConfig.NetworkMode.IsUserDefined() {
		return nil
	}

	for _, l := range hostConfig.Links {
		name, alias, err := opts.ParseLink(l)
		if err != nil {
			return err
		}
		child, err := daemon.GetContainer(name)
		if err != nil {
			if errdefs.IsNotFound(err) {
				// Trying to link to a non-existing container is not valid, and
				// should return an "invalid parameter" error. Returning a "not
				// found" error here would make the client report the container's
				// image could not be found (see moby/moby#39823)
				err = errdefs.InvalidParameter(err)
			}
			return errors.Wrapf(err, "could not get container for %s", name)
		}
		for child.HostConfig.NetworkMode.IsContainer() {
			cid := child.HostConfig.NetworkMode.ConnectedContainer()
			child, err = daemon.GetContainer(cid)
			if err != nil {
				if errdefs.IsNotFound(err) {
					// Trying to link to a non-existing container is not valid, and
					// should return an "invalid parameter" error. Returning a "not
					// found" error here would make the client report the container's
					// image could not be found (see moby/moby#39823)
					err = errdefs.InvalidParameter(err)
				}
				return errors.Wrapf(err, "could not get container for %s", cid)
			}
		}
		if child.HostConfig.NetworkMode.IsHost() {
			return runconfig.ErrConflictHostNetworkAndLinks
		}
		if err := daemon.registerLink(container, child, alias); err != nil {
			return err
		}
	}

	return nil
}

// conditionalMountOnStart is a platform specific helper function during the
// container start to call mount.
func (daemon *Daemon) conditionalMountOnStart(container *container.Container) error {
	if !daemon.UsesSnapshotter() {
		return daemon.Mount(container)
	}
	return nil
}

// conditionalUnmountOnCleanup is a platform specific helper function called
// during the cleanup of a container to unmount.
func (daemon *Daemon) conditionalUnmountOnCleanup(container *container.Container) error {
	if !daemon.UsesSnapshotter() {
		return daemon.Unmount(container)
	}
	return nil
}

// setDefaultIsolation determines the default isolation mode for the
// daemon to run in. This is only applicable on Windows
func (daemon *Daemon) setDefaultIsolation() error {
	return nil
}

// setupDaemonProcess sets various settings for the daemon's process
func setupDaemonProcess(config *config.Config) error {
	// setup the daemons oom_score_adj
	if err := setupOOMScoreAdj(config.OOMScoreAdjust); err != nil {
		return err
	}
	if err := setMayDetachMounts(); err != nil {
		logrus.WithError(err).Warn("Could not set may_detach_mounts kernel parameter")
	}
	return nil
}

// This is used to allow removal of mountpoints that may be mounted in other
// namespaces on RHEL based kernels starting from RHEL 7.4.
// Without this setting, removals on these RHEL based kernels may fail with
// "device or resource busy".
// This setting is not available in upstream kernels as it is not configurable,
// but has been in the upstream kernels since 3.15.
func setMayDetachMounts() error {
	f, err := os.OpenFile("/proc/sys/fs/may_detach_mounts", os.O_WRONLY, 0)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errors.Wrap(err, "error opening may_detach_mounts kernel config file")
	}
	defer f.Close()

	_, err = f.WriteString("1")
	if os.IsPermission(err) {
		// Setting may_detach_mounts does not work in an
		// unprivileged container. Ignore the error, but log
		// it if we appear not to be in that situation.
		if !userns.RunningInUserNS() {
			logrus.Debugf("Permission denied writing %q to /proc/sys/fs/may_detach_mounts", "1")
		}
		return nil
	}
	return err
}

func setupOOMScoreAdj(score int) error {
	if score == 0 {
		return nil
	}
	f, err := os.OpenFile("/proc/self/oom_score_adj", os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	stringScore := strconv.Itoa(score)
	_, err = f.WriteString(stringScore)
	if os.IsPermission(err) {
		// Setting oom_score_adj does not work in an
		// unprivileged container. Ignore the error, but log
		// it if we appear not to be in that situation.
		if !userns.RunningInUserNS() {
			logrus.Debugf("Permission denied writing %q to /proc/self/oom_score_adj", stringScore)
		}
		return nil
	}

	return err
}

func (daemon *Daemon) initCPURtController(mnt, path string) error {
	if path == "/" || path == "." {
		return nil
	}

	// Recursively create cgroup to ensure that the system and all parent cgroups have values set
	// for the period and runtime as this limits what the children can be set to.
	if err := daemon.initCPURtController(mnt, filepath.Dir(path)); err != nil {
		return err
	}

	path = filepath.Join(mnt, path)
	if err := os.MkdirAll(path, 0755); err != nil {
		return err
	}
	if err := maybeCreateCPURealTimeFile(daemon.configStore.CPURealtimePeriod, "cpu.rt_period_us", path); err != nil {
		return err
	}
	return maybeCreateCPURealTimeFile(daemon.configStore.CPURealtimeRuntime, "cpu.rt_runtime_us", path)
}

func maybeCreateCPURealTimeFile(configValue int64, file string, path string) error {
	if configValue == 0 {
		return nil
	}
	return os.WriteFile(filepath.Join(path, file), []byte(strconv.FormatInt(configValue, 10)), 0700)
}

func (daemon *Daemon) setupSeccompProfile() error {
	switch profile := daemon.configStore.SeccompProfile; profile {
	case "", config.SeccompProfileDefault:
		daemon.seccompProfilePath = config.SeccompProfileDefault
	case config.SeccompProfileUnconfined:
		daemon.seccompProfilePath = config.SeccompProfileUnconfined
	default:
		daemon.seccompProfilePath = profile
		b, err := os.ReadFile(profile)
		if err != nil {
			return fmt.Errorf("opening seccomp profile (%s) failed: %v", profile, err)
		}
		daemon.seccompProfile = b
	}
	return nil
}

func getSysInfo(daemon *Daemon) *sysinfo.SysInfo {
	var siOpts []sysinfo.Opt
	if daemon.getCgroupDriver() == cgroupSystemdDriver {
		if euid := os.Getenv("ROOTLESSKIT_PARENT_EUID"); euid != "" {
			siOpts = append(siOpts, sysinfo.WithCgroup2GroupPath("/user.slice/user-"+euid+".slice"))
		}
	}
	return sysinfo.New(siOpts...)
}

func (daemon *Daemon) initLibcontainerd(ctx context.Context) error {
	var err error
	daemon.containerd, err = remote.NewClient(
		ctx,
		daemon.containerdCli,
		filepath.Join(daemon.configStore.ExecRoot, "containerd"),
		daemon.configStore.ContainerdNamespace,
		daemon,
	)
	return err
}

func recursiveUnmount(target string) error {
	return mount.RecursiveUnmount(target)
}

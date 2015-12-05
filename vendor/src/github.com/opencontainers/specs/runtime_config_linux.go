package specs

import "os"

// LinuxStateDirectory holds the container's state information
const LinuxStateDirectory = "/run/opencontainer/containers"

// LinuxRuntimeSpec is the full specification for linux containers.
type LinuxRuntimeSpec struct {
	RuntimeSpec
	// LinuxRuntime is platform specific configuration for linux based containers.
	Linux LinuxRuntime `json:"linux"`
}

// LinuxRuntime hosts the Linux-only runtime information
type LinuxRuntime struct {
	// UIDMapping specifies user mappings for supporting user namespaces on linux.
	UIDMappings []IDMapping `json:"uidMappings"`
	// GIDMapping specifies group mappings for supporting user namespaces on linux.
	GIDMappings []IDMapping `json:"gidMappings"`
	// Rlimits specifies rlimit options to apply to the container's process.
	Rlimits []Rlimit `json:"rlimits"`
	// Sysctl are a set of key value pairs that are set for the container on start
	Sysctl map[string]string `json:"sysctl"`
	// Resources contain cgroup information for handling resource constraints
	// for the container
	Resources *Resources `json:"resources"`
	// CgroupsPath specifies the path to cgroups that are created and/or joined by the container.
	// The path is expected to be relative to the cgroups mountpoint.
	// If resources are specified, the cgroups at CgroupsPath will be updated based on resources.
	CgroupsPath string `json:"cgroupsPath"`
	// Namespaces contains the namespaces that are created and/or joined by the container
	Namespaces []Namespace `json:"namespaces"`
	// Devices are a list of device nodes that are created and enabled for the container
	Devices []Device `json:"devices"`
	// ApparmorProfile specified the apparmor profile for the container.
	ApparmorProfile string `json:"apparmorProfile"`
	// SelinuxProcessLabel specifies the selinux context that the container process is run as.
	SelinuxProcessLabel string `json:"selinuxProcessLabel"`
	// Seccomp specifies the seccomp security settings for the container.
	Seccomp Seccomp `json:"seccomp"`
	// RootfsPropagation is the rootfs mount propagation mode for the container
	RootfsPropagation string `json:"rootfsPropagation"`
}

// Namespace is the configuration for a linux namespace
type Namespace struct {
	// Type is the type of Linux namespace
	Type NamespaceType `json:"type"`
	// Path is a path to an existing namespace persisted on disk that can be joined
	// and is of the same type
	Path string `json:"path"`
}

// NamespaceType is one of the linux namespaces
type NamespaceType string

const (
	// PIDNamespace for isolating process IDs
	PIDNamespace NamespaceType = "pid"
	// NetworkNamespace for isolating network devices, stacks, ports, etc
	NetworkNamespace = "network"
	// MountNamespace for isolating mount points
	MountNamespace = "mount"
	// IPCNamespace for isolating System V IPC, POSIX message queues
	IPCNamespace = "ipc"
	// UTSNamespace for isolating hostname and NIS domain name
	UTSNamespace = "uts"
	// UserNamespace for isolating user and group IDs
	UserNamespace = "user"
)

// IDMapping specifies UID/GID mappings
type IDMapping struct {
	// HostID is the UID/GID of the host user or group
	HostID uint32 `json:"hostID"`
	// ContainerID is the UID/GID of the container's user or group
	ContainerID uint32 `json:"containerID"`
	// Size is the length of the range of IDs mapped between the two namespaces
	Size uint32 `json:"size"`
}

// Rlimit type and restrictions
type Rlimit struct {
	// Type of the rlimit to set
	Type string `json:"type"`
	// Hard is the hard limit for the specified type
	Hard uint64 `json:"hard"`
	// Soft is the soft limit for the specified type
	Soft uint64 `json:"soft"`
}

// HugepageLimit structure corresponds to limiting kernel hugepages
type HugepageLimit struct {
	// Pagesize is the hugepage size
	Pagesize string `json:"pageSize"`
	// Limit is the limit of "hugepagesize" hugetlb usage
	Limit uint64 `json:"limit"`
}

// InterfacePriority for network interfaces
type InterfacePriority struct {
	// Name is the name of the network interface
	Name string `json:"name"`
	// Priority for the interface
	Priority uint32 `json:"priority"`
}

// blockIODevice holds major:minor format supported in blkio cgroup
type blockIODevice struct {
	// Major is the device's major number.
	Major int64 `json:"major"`
	// Minor is the device's minor number.
	Minor int64 `json:"minor"`
}

// WeightDevice struct holds a `major:minor weight` pair for blkioWeightDevice
type WeightDevice struct {
	blockIODevice
	// Weight is the bandwidth rate for the device, range is from 10 to 1000
	Weight uint16 `json:"weight"`
	// LeafWeight is the bandwidth rate for the device while competing with the cgroup's child cgroups, range is from 10 to 1000, CFQ scheduler only
	LeafWeight uint16 `json:"leafWeight"`
}

// ThrottleDevice struct holds a `major:minor rate_per_second` pair
type ThrottleDevice struct {
	blockIODevice
	// Rate is the IO rate limit per cgroup per device
	Rate uint64 `json:"rate"`
}

// BlockIO for Linux cgroup 'blkio' resource management
type BlockIO struct {
	// Specifies per cgroup weight, range is from 10 to 1000
	Weight uint16 `json:"blkioWeight"`
	// Specifies tasks' weight in the given cgroup while competing with the cgroup's child cgroups, range is from 10 to 1000, CFQ scheduler only
	LeafWeight uint16 `json:"blkioLeafWeight"`
	// Weight per cgroup per device, can override BlkioWeight
	WeightDevice []*WeightDevice `json:"blkioWeightDevice"`
	// IO read rate limit per cgroup per device, bytes per second
	ThrottleReadBpsDevice []*ThrottleDevice `json:"blkioThrottleReadBpsDevice"`
	// IO write rate limit per cgroup per device, bytes per second
	ThrottleWriteBpsDevice []*ThrottleDevice `json:"blkioThrottleWriteBpsDevice"`
	// IO read rate limit per cgroup per device, IO per second
	ThrottleReadIOPSDevice []*ThrottleDevice `json:"blkioThrottleReadIOPSDevice"`
	// IO write rate limit per cgroup per device, IO per second
	ThrottleWriteIOPSDevice []*ThrottleDevice `json:"blkioThrottleWriteIOPSDevice"`
}

// Memory for Linux cgroup 'memory' resource management
type Memory struct {
	// Memory limit (in bytes)
	Limit uint64 `json:"limit"`
	// Memory reservation or soft_limit (in bytes)
	Reservation uint64 `json:"reservation"`
	// Total memory usage (memory + swap); set `-1' to disable swap
	Swap uint64 `json:"swap"`
	// Kernel memory limit (in bytes)
	Kernel uint64 `json:"kernel"`
	// How aggressive the kernel will swap memory pages. Range from 0 to 100. Set -1 to use system default
	Swappiness uint64 `json:"swappiness"`
}

// CPU for Linux cgroup 'cpu' resource management
type CPU struct {
	// CPU shares (relative weight vs. other cgroups with cpu shares)
	Shares uint64 `json:"shares"`
	// CPU hardcap limit (in usecs). Allowed cpu time in a given period
	Quota uint64 `json:"quota"`
	// CPU period to be used for hardcapping (in usecs). 0 to use system default
	Period uint64 `json:"period"`
	// How many time CPU will use in realtime scheduling (in usecs)
	RealtimeRuntime uint64 `json:"realtimeRuntime"`
	// CPU period to be used for realtime scheduling (in usecs)
	RealtimePeriod uint64 `json:"realtimePeriod"`
	// CPU to use within the cpuset
	Cpus string `json:"cpus"`
	// MEM to use within the cpuset
	Mems string `json:"mems"`
}

// Pids for Linux cgroup 'pids' resource management (Linux 4.3)
type Pids struct {
	// Maximum number of PIDs. A value <= 0 indicates "no limit".
	Limit int64 `json:"limit"`
}

// Network identification and priority configuration
type Network struct {
	// Set class identifier for container's network packets
	// this is actually a string instead of a uint64 to overcome the json
	// limitation of specifying hex numbers
	ClassID string `json:"classID"`
	// Set priority of network traffic for container
	Priorities []InterfacePriority `json:"priorities"`
}

// Resources has container runtime resource constraints
type Resources struct {
	// DisableOOMKiller disables the OOM killer for out of memory conditions
	DisableOOMKiller bool `json:"disableOOMKiller"`
	// Specify an oom_score_adj for the container. Optional.
	OOMScoreAdj int `json:"oomScoreAdj"`
	// Memory restriction configuration
	Memory Memory `json:"memory"`
	// CPU resource restriction configuration
	CPU CPU `json:"cpu"`
	// Task resource restriction configuration.
	Pids Pids `json:"pids"`
	// BlockIO restriction configuration
	BlockIO BlockIO `json:"blockIO"`
	// Hugetlb limit (in bytes)
	HugepageLimits []HugepageLimit `json:"hugepageLimits"`
	// Network restriction configuration
	Network Network `json:"network"`
}

// Device represents the information on a Linux special device file
type Device struct {
	// Path to the device.
	Path string `json:"path"`
	// Device type, block, char, etc.
	Type rune `json:"type"`
	// Major is the device's major number.
	Major int64 `json:"major"`
	// Minor is the device's minor number.
	Minor int64 `json:"minor"`
	// Cgroup permissions format, rwm.
	Permissions string `json:"permissions"`
	// FileMode permission bits for the device.
	FileMode os.FileMode `json:"fileMode"`
	// UID of the device.
	UID uint32 `json:"uid"`
	// Gid of the device.
	GID uint32 `json:"gid"`
}

// Seccomp represents syscall restrictions
type Seccomp struct {
	DefaultAction Action     `json:"defaultAction"`
	Architectures []Arch     `json:"architectures"`
	Syscalls      []*Syscall `json:"syscalls"`
}

// Additional architectures permitted to be used for system calls
// By default only the native architecture of the kernel is permitted
type Arch string

const (
	ArchX86         Arch = "SCMP_ARCH_X86"
	ArchX86_64      Arch = "SCMP_ARCH_X86_64"
	ArchX32         Arch = "SCMP_ARCH_X32"
	ArchARM         Arch = "SCMP_ARCH_ARM"
	ArchAARCH64     Arch = "SCMP_ARCH_AARCH64"
	ArchMIPS        Arch = "SCMP_ARCH_MIPS"
	ArchMIPS64      Arch = "SCMP_ARCH_MIPS64"
	ArchMIPS64N32   Arch = "SCMP_ARCH_MIPS64N32"
	ArchMIPSEL      Arch = "SCMP_ARCH_MIPSEL"
	ArchMIPSEL64    Arch = "SCMP_ARCH_MIPSEL64"
	ArchMIPSEL64N32 Arch = "SCMP_ARCH_MIPSEL64N32"
)

// Action taken upon Seccomp rule match
type Action string

const (
	ActKill  Action = "SCMP_ACT_KILL"
	ActTrap  Action = "SCMP_ACT_TRAP"
	ActErrno Action = "SCMP_ACT_ERRNO"
	ActTrace Action = "SCMP_ACT_TRACE"
	ActAllow Action = "SCMP_ACT_ALLOW"
)

// Operator used to match syscall arguments in Seccomp
type Operator string

const (
	OpNotEqual     Operator = "SCMP_CMP_NE"
	OpLessThan     Operator = "SCMP_CMP_LT"
	OpLessEqual    Operator = "SCMP_CMP_LE"
	OpEqualTo      Operator = "SCMP_CMP_EQ"
	OpGreaterEqual Operator = "SCMP_CMP_GE"
	OpGreaterThan  Operator = "SCMP_CMP_GT"
	OpMaskedEqual  Operator = "SCMP_CMP_MASKED_EQ"
)

// Arg used for matching specific syscall arguments in Seccomp
type Arg struct {
	Index    uint     `json:"index"`
	Value    uint64   `json:"value"`
	ValueTwo uint64   `json:"valueTwo"`
	Op       Operator `json:"op"`
}

// Syscall is used to match a syscall in Seccomp
type Syscall struct {
	Name   string `json:"name"`
	Action Action `json:"action"`
	Args   []*Arg `json:"args"`
}

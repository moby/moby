package specs

import "os"

// Spec is the base configuration for the container.
type Spec struct {
	// Version of the Open Container Runtime Specification with which the bundle complies.
	Version string `json:"ociVersion"`
	// Platform specifies the configuration's target platform.
	Platform Platform `json:"platform"`
	// Process configures the container process.
	Process Process `json:"process"`
	// Root configures the container's root filesystem.
	Root Root `json:"root"`
	// Hostname configures the container's hostname.
	Hostname string `json:"hostname,omitempty"`
	// Mounts configures additional mounts (on top of Root).
	Mounts []Mount `json:"mounts,omitempty"`
	// Hooks configures callbacks for container lifecycle events.
	Hooks Hooks `json:"hooks"`
	// Annotations contains arbitrary metadata for the container.
	Annotations map[string]string `json:"annotations,omitempty"`

	// Linux is platform specific configuration for Linux based containers.
	Linux *Linux `json:"linux,omitempty" platform:"linux"`
	// Solaris is platform specific configuration for Solaris containers.
	Solaris *Solaris `json:"solaris,omitempty" platform:"solaris"`
	// Windows is platform specific configuration for Windows based containers, including Hyper-V containers.
	Windows *Windows `json:"windows,omitempty" platform:"windows"`
}

// Process contains information to start a specific application inside the container.
type Process struct {
	// Terminal creates an interactive terminal for the container.
	Terminal bool `json:"terminal,omitempty"`
	// ConsoleSize specifies the size of the console.
	ConsoleSize Box `json:"consoleSize,omitempty"`
	// User specifies user information for the process.
	User User `json:"user"`
	// Args specifies the binary and arguments for the application to execute.
	Args []string `json:"args"`
	// Env populates the process environment for the process.
	Env []string `json:"env,omitempty"`
	// Cwd is the current working directory for the process and must be
	// relative to the container's root.
	Cwd string `json:"cwd"`
	// Capabilities are Linux capabilities that are kept for the container.
	Capabilities []string `json:"capabilities,omitempty" platform:"linux"`
	// Rlimits specifies rlimit options to apply to the process.
	Rlimits []Rlimit `json:"rlimits,omitempty" platform:"linux"`
	// NoNewPrivileges controls whether additional privileges could be gained by processes in the container.
	NoNewPrivileges bool `json:"noNewPrivileges,omitempty" platform:"linux"`
	// ApparmorProfile specifies the apparmor profile for the container.
	ApparmorProfile string `json:"apparmorProfile,omitempty" platform:"linux"`
	// SelinuxLabel specifies the selinux context that the container process is run as.
	SelinuxLabel string `json:"selinuxLabel,omitempty" platform:"linux"`
}

// Box specifies dimensions of a rectangle. Used for specifying the size of a console.
type Box struct {
	// Height is the vertical dimension of a box.
	Height uint `json:"height"`
	// Width is the horizontal dimension of a box.
	Width uint `json:"width"`
}

// User specifies specific user (and group) information for the container process.
type User struct {
	// UID is the user id.
	UID uint32 `json:"uid" platform:"linux,solaris"`
	// GID is the group id.
	GID uint32 `json:"gid" platform:"linux,solaris"`
	// AdditionalGids are additional group ids set for the container's process.
	AdditionalGids []uint32 `json:"additionalGids,omitempty" platform:"linux,solaris"`
	// Username is the user name.
	Username string `json:"username,omitempty" platform:"windows"`
}

// Root contains information about the container's root filesystem on the host.
type Root struct {
	// Path is the absolute path to the container's root filesystem.
	Path string `json:"path"`
	// Readonly makes the root filesystem for the container readonly before the process is executed.
	Readonly bool `json:"readonly,omitempty"`
}

// Platform specifies OS and arch information for the host system that the container
// is created for.
type Platform struct {
	// OS is the operating system.
	OS string `json:"os"`
	// Arch is the architecture
	Arch string `json:"arch"`
}

// Mount specifies a mount for a container.
type Mount struct {
	// Destination is the path where the mount will be placed relative to the container's root.  The path and child directories MUST exist, a runtime MUST NOT create directories automatically to a mount point.
	Destination string `json:"destination"`
	// Type specifies the mount kind.
	Type string `json:"type"`
	// Source specifies the source path of the mount.  In the case of bind mounts on
	// Linux based systems this would be the file on the host.
	Source string `json:"source"`
	// Options are fstab style mount options.
	Options []string `json:"options,omitempty"`
}

// Hook specifies a command that is run at a particular event in the lifecycle of a container
type Hook struct {
	Path    string   `json:"path"`
	Args    []string `json:"args,omitempty"`
	Env     []string `json:"env,omitempty"`
	Timeout *int     `json:"timeout,omitempty"`
}

// Hooks for container setup and teardown
type Hooks struct {
	// Prestart is a list of hooks to be run before the container process is executed.
	// On Linux, they are run after the container namespaces are created.
	Prestart []Hook `json:"prestart,omitempty"`
	// Poststart is a list of hooks to be run after the container process is started.
	Poststart []Hook `json:"poststart,omitempty"`
	// Poststop is a list of hooks to be run after the container process exits.
	Poststop []Hook `json:"poststop,omitempty"`
}

// Linux contains platform specific configuration for Linux based containers.
type Linux struct {
	// UIDMapping specifies user mappings for supporting user namespaces on Linux.
	UIDMappings []IDMapping `json:"uidMappings,omitempty"`
	// GIDMapping specifies group mappings for supporting user namespaces on Linux.
	GIDMappings []IDMapping `json:"gidMappings,omitempty"`
	// Sysctl are a set of key value pairs that are set for the container on start
	Sysctl map[string]string `json:"sysctl,omitempty"`
	// Resources contain cgroup information for handling resource constraints
	// for the container
	Resources *Resources `json:"resources,omitempty"`
	// CgroupsPath specifies the path to cgroups that are created and/or joined by the container.
	// The path is expected to be relative to the cgroups mountpoint.
	// If resources are specified, the cgroups at CgroupsPath will be updated based on resources.
	CgroupsPath *string `json:"cgroupsPath,omitempty"`
	// Namespaces contains the namespaces that are created and/or joined by the container
	Namespaces []Namespace `json:"namespaces,omitempty"`
	// Devices are a list of device nodes that are created for the container
	Devices []Device `json:"devices,omitempty"`
	// Seccomp specifies the seccomp security settings for the container.
	Seccomp *Seccomp `json:"seccomp,omitempty"`
	// RootfsPropagation is the rootfs mount propagation mode for the container.
	RootfsPropagation string `json:"rootfsPropagation,omitempty"`
	// MaskedPaths masks over the provided paths inside the container.
	MaskedPaths []string `json:"maskedPaths,omitempty"`
	// ReadonlyPaths sets the provided paths as RO inside the container.
	ReadonlyPaths []string `json:"readonlyPaths,omitempty"`
	// MountLabel specifies the selinux context for the mounts in the container.
	MountLabel string `json:"mountLabel,omitempty"`
}

// Namespace is the configuration for a Linux namespace
type Namespace struct {
	// Type is the type of Linux namespace
	Type NamespaceType `json:"type"`
	// Path is a path to an existing namespace persisted on disk that can be joined
	// and is of the same type
	Path string `json:"path,omitempty"`
}

// NamespaceType is one of the Linux namespaces
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
	// CgroupNamespace for isolating cgroup hierarchies
	CgroupNamespace = "cgroup"
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
	Pagesize *string `json:"pageSize,omitempty"`
	// Limit is the limit of "hugepagesize" hugetlb usage
	Limit *uint64 `json:"limit,omitempty"`
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
	Weight *uint16 `json:"weight,omitempty"`
	// LeafWeight is the bandwidth rate for the device while competing with the cgroup's child cgroups, range is from 10 to 1000, CFQ scheduler only
	LeafWeight *uint16 `json:"leafWeight,omitempty"`
}

// ThrottleDevice struct holds a `major:minor rate_per_second` pair
type ThrottleDevice struct {
	blockIODevice
	// Rate is the IO rate limit per cgroup per device
	Rate *uint64 `json:"rate,omitempty"`
}

// BlockIO for Linux cgroup 'blkio' resource management
type BlockIO struct {
	// Specifies per cgroup weight, range is from 10 to 1000
	Weight *uint16 `json:"blkioWeight,omitempty"`
	// Specifies tasks' weight in the given cgroup while competing with the cgroup's child cgroups, range is from 10 to 1000, CFQ scheduler only
	LeafWeight *uint16 `json:"blkioLeafWeight,omitempty"`
	// Weight per cgroup per device, can override BlkioWeight
	WeightDevice []WeightDevice `json:"blkioWeightDevice,omitempty"`
	// IO read rate limit per cgroup per device, bytes per second
	ThrottleReadBpsDevice []ThrottleDevice `json:"blkioThrottleReadBpsDevice,omitempty"`
	// IO write rate limit per cgroup per device, bytes per second
	ThrottleWriteBpsDevice []ThrottleDevice `json:"blkioThrottleWriteBpsDevice,omitempty"`
	// IO read rate limit per cgroup per device, IO per second
	ThrottleReadIOPSDevice []ThrottleDevice `json:"blkioThrottleReadIOPSDevice,omitempty"`
	// IO write rate limit per cgroup per device, IO per second
	ThrottleWriteIOPSDevice []ThrottleDevice `json:"blkioThrottleWriteIOPSDevice,omitempty"`
}

// Memory for Linux cgroup 'memory' resource management
type Memory struct {
	// Memory limit (in bytes).
	Limit *uint64 `json:"limit,omitempty"`
	// Memory reservation or soft_limit (in bytes).
	Reservation *uint64 `json:"reservation,omitempty"`
	// Total memory limit (memory + swap).
	Swap *uint64 `json:"swap,omitempty"`
	// Kernel memory limit (in bytes).
	Kernel *uint64 `json:"kernel,omitempty"`
	// Kernel memory limit for tcp (in bytes)
	KernelTCP *uint64 `json:"kernelTCP,omitempty"`
	// How aggressive the kernel will swap memory pages. Range from 0 to 100.
	Swappiness *uint64 `json:"swappiness,omitempty"`
}

// CPU for Linux cgroup 'cpu' resource management
type CPU struct {
	// CPU shares (relative weight (ratio) vs. other cgroups with cpu shares).
	Shares *uint64 `json:"shares,omitempty"`
	// CPU hardcap limit (in usecs). Allowed cpu time in a given period.
	Quota *uint64 `json:"quota,omitempty"`
	// CPU period to be used for hardcapping (in usecs).
	Period *uint64 `json:"period,omitempty"`
	// How much time realtime scheduling may use (in usecs).
	RealtimeRuntime *uint64 `json:"realtimeRuntime,omitempty"`
	// CPU period to be used for realtime scheduling (in usecs).
	RealtimePeriod *uint64 `json:"realtimePeriod,omitempty"`
	// CPUs to use within the cpuset. Default is to use any CPU available.
	Cpus *string `json:"cpus,omitempty"`
	// List of memory nodes in the cpuset. Default is to use any available memory node.
	Mems *string `json:"mems,omitempty"`
}

// Pids for Linux cgroup 'pids' resource management (Linux 4.3)
type Pids struct {
	// Maximum number of PIDs. Default is "no limit".
	Limit *int64 `json:"limit,omitempty"`
}

// Network identification and priority configuration
type Network struct {
	// Set class identifier for container's network packets
	ClassID *uint32 `json:"classID,omitempty"`
	// Set priority of network traffic for container
	Priorities []InterfacePriority `json:"priorities,omitempty"`
}

// Resources has container runtime resource constraints
type Resources struct {
	// Devices configures the device whitelist.
	Devices []DeviceCgroup `json:"devices,omitempty"`
	// DisableOOMKiller disables the OOM killer for out of memory conditions
	DisableOOMKiller *bool `json:"disableOOMKiller,omitempty"`
	// Specify an oom_score_adj for the container.
	OOMScoreAdj *int `json:"oomScoreAdj,omitempty"`
	// Memory restriction configuration
	Memory *Memory `json:"memory,omitempty"`
	// CPU resource restriction configuration
	CPU *CPU `json:"cpu,omitempty"`
	// Task resource restriction configuration.
	Pids *Pids `json:"pids,omitempty"`
	// BlockIO restriction configuration
	BlockIO *BlockIO `json:"blockIO,omitempty"`
	// Hugetlb limit (in bytes)
	HugepageLimits []HugepageLimit `json:"hugepageLimits,omitempty"`
	// Network restriction configuration
	Network *Network `json:"network,omitempty"`
}

// Device represents the mknod information for a Linux special device file
type Device struct {
	// Path to the device.
	Path string `json:"path"`
	// Device type, block, char, etc.
	Type string `json:"type"`
	// Major is the device's major number.
	Major int64 `json:"major"`
	// Minor is the device's minor number.
	Minor int64 `json:"minor"`
	// FileMode permission bits for the device.
	FileMode *os.FileMode `json:"fileMode,omitempty"`
	// UID of the device.
	UID *uint32 `json:"uid,omitempty"`
	// Gid of the device.
	GID *uint32 `json:"gid,omitempty"`
}

// DeviceCgroup represents a device rule for the whitelist controller
type DeviceCgroup struct {
	// Allow or deny
	Allow bool `json:"allow"`
	// Device type, block, char, etc.
	Type *string `json:"type,omitempty"`
	// Major is the device's major number.
	Major *int64 `json:"major,omitempty"`
	// Minor is the device's minor number.
	Minor *int64 `json:"minor,omitempty"`
	// Cgroup access permissions format, rwm.
	Access *string `json:"access,omitempty"`
}

// Seccomp represents syscall restrictions
type Seccomp struct {
	DefaultAction Action    `json:"defaultAction"`
	Architectures []Arch    `json:"architectures"`
	Syscalls      []Syscall `json:"syscalls,omitempty"`
}

// Solaris contains platform specific configuration for Solaris application containers.
type Solaris struct {
	// SMF FMRI which should go "online" before we start the container process.
	Milestone string `json:"milestone,omitempty"`
	// Maximum set of privileges any process in this container can obtain.
	LimitPriv string `json:"limitpriv,omitempty"`
	// The maximum amount of shared memory allowed for this container.
	MaxShmMemory string `json:"maxShmMemory,omitempty"`
	// Specification for automatic creation of network resources for this container.
	Anet []Anet `json:"anet,omitempty"`
	// Set limit on the amount of CPU time that can be used by container.
	CappedCPU *CappedCPU `json:"cappedCPU,omitempty"`
	// The physical and swap caps on the memory that can be used by this container.
	CappedMemory *CappedMemory `json:"cappedMemory,omitempty"`
}

// CappedCPU allows users to set limit on the amount of CPU time that can be used by container.
type CappedCPU struct {
	Ncpus string `json:"ncpus,omitempty"`
}

// CappedMemory allows users to set the physical and swap caps on the memory that can be used by this container.
type CappedMemory struct {
	Physical string `json:"physical,omitempty"`
	Swap     string `json:"swap,omitempty"`
}

// Anet provides the specification for automatic creation of network resources for this container.
type Anet struct {
	// Specify a name for the automatically created VNIC datalink.
	Linkname string `json:"linkname,omitempty"`
	// Specify the link over which the VNIC will be created.
	Lowerlink string `json:"lowerLink,omitempty"`
	// The set of IP addresses that the container can use.
	Allowedaddr string `json:"allowedAddress,omitempty"`
	// Specifies whether allowedAddress limitation is to be applied to the VNIC.
	Configallowedaddr string `json:"configureAllowedAddress,omitempty"`
	// The value of the optional default router.
	Defrouter string `json:"defrouter,omitempty"`
	// Enable one or more types of link protection.
	Linkprotection string `json:"linkProtection,omitempty"`
	// Set the VNIC's macAddress
	Macaddress string `json:"macAddress,omitempty"`
}

// Windows defines the runtime configuration for Windows based containers, including Hyper-V containers.
type Windows struct {
	// Resources contains information for handling resource constraints for the container.
	Resources *WindowsResources `json:"resources,omitempty"`
}

// WindowsResources has container runtime resource constraints for containers running on Windows.
type WindowsResources struct {
	// Memory restriction configuration.
	Memory *WindowsMemoryResources `json:"memory,omitempty"`
	// CPU resource restriction configuration.
	CPU *WindowsCPUResources `json:"cpu,omitempty"`
	// Storage restriction configuration.
	Storage *WindowsStorageResources `json:"storage,omitempty"`
	// Network restriction configuration.
	Network *WindowsNetworkResources `json:"network,omitempty"`
}

// WindowsMemoryResources contains memory resource management settings.
type WindowsMemoryResources struct {
	// Memory limit in bytes.
	Limit *uint64 `json:"limit,omitempty"`
	// Memory reservation in bytes.
	Reservation *uint64 `json:"reservation,omitempty"`
}

// WindowsCPUResources contains CPU resource management settings.
type WindowsCPUResources struct {
	// Number of CPUs available to the container.
	Count *uint64 `json:"count,omitempty"`
	// CPU shares (relative weight to other containers with cpu shares). Range is from 1 to 10000.
	Shares *uint16 `json:"shares,omitempty"`
	// Percent of available CPUs usable by the container.
	Percent *uint8 `json:"percent,omitempty"`
}

// WindowsStorageResources contains storage resource management settings.
type WindowsStorageResources struct {
	// Specifies maximum Iops for the system drive.
	Iops *uint64 `json:"iops,omitempty"`
	// Specifies maximum bytes per second for the system drive.
	Bps *uint64 `json:"bps,omitempty"`
	// Sandbox size specifies the minimum size of the system drive in bytes.
	SandboxSize *uint64 `json:"sandboxSize,omitempty"`
}

// WindowsNetworkResources contains network resource management settings.
type WindowsNetworkResources struct {
	// EgressBandwidth is the maximum egress bandwidth in bytes per second.
	EgressBandwidth *uint64 `json:"egressBandwidth,omitempty"`
}

// Arch used for additional architectures
type Arch string

// Additional architectures permitted to be used for system calls
// By default only the native architecture of the kernel is permitted
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
	ArchPPC         Arch = "SCMP_ARCH_PPC"
	ArchPPC64       Arch = "SCMP_ARCH_PPC64"
	ArchPPC64LE     Arch = "SCMP_ARCH_PPC64LE"
	ArchS390        Arch = "SCMP_ARCH_S390"
	ArchS390X       Arch = "SCMP_ARCH_S390X"
)

// Action taken upon Seccomp rule match
type Action string

// Define actions for Seccomp rules
const (
	ActKill  Action = "SCMP_ACT_KILL"
	ActTrap  Action = "SCMP_ACT_TRAP"
	ActErrno Action = "SCMP_ACT_ERRNO"
	ActTrace Action = "SCMP_ACT_TRACE"
	ActAllow Action = "SCMP_ACT_ALLOW"
)

// Operator used to match syscall arguments in Seccomp
type Operator string

// Define operators for syscall arguments in Seccomp
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
	Args   []Arg  `json:"args,omitempty"`
}

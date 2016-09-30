package windowsoci

// This file contains the Windows spec for a container. At the time of
// writing, Windows does not have a spec defined in opencontainers/specs,
// hence this is an interim workaround. TODO Windows: FIXME @jhowardmsft

import "fmt"

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

// Windows contains platform specific configuration for Windows based containers.
type Windows struct {
	// Resources contains information for handling resource constraints for the container
	Resources *WindowsResources `json:"resources,omitempty"`
}

// Process contains information to start a specific application inside the container.
type Process struct {
	// Terminal creates an interactive terminal for the container.
	Terminal bool `json:"terminal,omitempty"`
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
	// ConsoleSize contains the initial size of the console.
	ConsoleSize Box `json:"consoleSize" platform:"windows"`
}

// Box specifies height and width dimensions. Used for sizing of a console.
type Box struct {
	Height uint
	Width  uint
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
	Readonly bool `json:"readonly"`
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

// WindowsStorage contains storage resource management settings
type WindowsStorage struct {
	// Specifies maximum Iops for the system drive
	Iops *uint64 `json:"iops,omitempty"`
	// Specifies maximum bytes per second for the system drive
	Bps *uint64 `json:"bps,omitempty"`
	// Sandbox size indicates the size to expand the system drive to if it is currently smaller
	SandboxSize *uint64 `json:"sandbox_size,omitempty"`
}

// WindowsMemory contains memory settings for the container
type WindowsMemory struct {
	// Memory limit (in bytes).
	Limit *int64 `json:"limit,omitempty"`
	// Memory reservation (in bytes).
	Reservation *uint64 `json:"reservation,omitempty"`
}

// WindowsCPU contains information for cpu resource management
type WindowsCPU struct {
	// Number of CPUs available to the container. This is an appoximation for Windows Server Containers.
	Count *uint64 `json:"count,omitempty"`
	// CPU shares (relative weight (ratio) vs. other containers with cpu shares). Range is from 1 to 10000.
	Shares *uint64 `json:"shares,omitempty"`
	// Percent of available CPUs usable by the container.
	Percent *int64 `json:"percent,omitempty"`
}

// WindowsNetwork contains network resource management information
type WindowsNetwork struct {
	// Bandwidth is the maximum egress bandwidth in bytes per second
	Bandwidth *uint64 `json:"bandwidth,omitempty"`
}

// WindowsResources has container runtime resource constraints
// TODO Windows containerd. This structure needs ratifying with the old resources
// structure used on Windows and the latest OCI spec.
type WindowsResources struct {
	// Memory restriction configuration
	Memory *WindowsMemory `json:"memory,omitempty"`
	// CPU resource restriction configuration
	CPU *WindowsCPU `json:"cpu,omitempty"`
	// Storage restriction configuration
	Storage *WindowsStorage `json:"storage,omitempty"`
	// Network restriction configuration
	Network *WindowsNetwork `json:"network,omitempty"`
}

const (
	// VersionMajor is for an API incompatible changes
	VersionMajor = 0
	// VersionMinor is for functionality in a backwards-compatible manner
	VersionMinor = 3
	// VersionPatch is for backwards-compatible bug fixes
	VersionPatch = 0

	// VersionDev indicates development branch. Releases will be empty string.
	VersionDev = ""
)

// Version is the specification version that the package types support.
var Version = fmt.Sprintf("%d.%d.%d%s (Windows)", VersionMajor, VersionMinor, VersionPatch, VersionDev)

//
// Temporary structures. Ultimately this whole file will be removed.
//

// Linux contains platform specific configuration for Linux based containers.
type Linux struct {
}

// Solaris contains platform specific configuration for Solaris application containers.
type Solaris struct {
}

// Hooks for container setup and teardown
type Hooks struct {
}

// Rlimit type and restrictions. Placeholder only to support the Process structure.
// Not used on Windows, only present for compilation purposes.
type Rlimit struct {
}

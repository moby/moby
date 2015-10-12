package execdriver

import (
	"errors"
	"io"
	"os/exec"
	"time"

	"github.com/docker/docker/pkg/idtools"
	// TODO Windows: Factor out ulimit
	"github.com/docker/docker/pkg/ulimit"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/configs"
)

// Context is a generic key value pair that allows
// arbatrary data to be sent
type Context map[string]string

// Define error messages
var (
	ErrNotRunning              = errors.New("Container is not running")
	ErrWaitTimeoutReached      = errors.New("Wait timeout reached")
	ErrDriverAlreadyRegistered = errors.New("A driver already registered this docker init function")
	ErrDriverNotFound          = errors.New("The requested docker init has not been found")
)

// DriverCallback defines a callback function which is used in "Run" and "Exec".
// This allows work to be done in the parent process when the child is passing
// through PreStart, Start and PostStop events.
// Callbacks are provided a processConfig pointer and the pid of the child.
// The channel will be used to notify the OOM events.
type DriverCallback func(processConfig *ProcessConfig, pid int, chOOM <-chan struct{}) error

// Hooks is a struct containing function pointers to callbacks
// used by any execdriver implementation exploiting hooks capabilities
type Hooks struct {
	// PreStart is called before container's CMD/ENTRYPOINT is executed
	PreStart []DriverCallback
	// Start is called after the container's process is full started
	Start DriverCallback
	// PostStop is called after the container process exits
	PostStop []DriverCallback
}

// Info is driver specific information based on
// processes registered with the driver
type Info interface {
	IsRunning() bool
}

// Terminal represents a pseudo TTY, it is for when
// using a container interactively.
type Terminal interface {
	io.Closer
	Resize(height, width int) error
}

// ExitStatus provides exit reasons for a container.
type ExitStatus struct {
	// The exit code with which the container exited.
	ExitCode int

	// Whether the container encountered an OOM.
	OOMKilled bool
}

// Driver is an interface for drivers to implement
// including all basic functions a driver should have
type Driver interface {
	// Run executes the process, blocks until the process exits and returns
	// the exit code. It's the last stage on Docker side for running a container.
	Run(c *Command, pipes *Pipes, hooks Hooks) (ExitStatus, error)

	// Exec executes the process in an existing container, blocks until the
	// process exits and returns the exit code.
	Exec(c *Command, processConfig *ProcessConfig, pipes *Pipes, hooks Hooks) (int, error)

	// Kill sends signals to process in container.
	Kill(c *Command, sig int) error

	// Pause pauses a container.
	Pause(c *Command) error

	// Unpause unpauses a container.
	Unpause(c *Command) error

	// Name returns the name of the driver.
	Name() string

	// Info returns the configuration stored in the driver struct,
	// "temporary" hack (until we move state from core to plugins).
	Info(id string) Info

	// GetPidsForContainer returns a list of pid for the processes running in a container.
	GetPidsForContainer(id string) ([]int, error)

	// Terminate kills a container by sending signal SIGKILL.
	Terminate(c *Command) error

	// Clean removes all traces of container exec.
	Clean(id string) error

	// Stats returns resource stats for a running container
	Stats(id string) (*ResourceStats, error)

	// SupportsHooks refers to the driver capability to exploit pre/post hook functionality
	SupportsHooks() bool
}

// Ipc settings of the container
// It is for IPC namespace setting. Usually different containers
// have their own IPC namespace, however this specifies to use
// an existing IPC namespace.
// You can join the host's or a container's IPC namespace.
type Ipc struct {
	ContainerID string `json:"container_id"` // id of the container to join ipc.
	HostIpc     bool   `json:"host_ipc"`
}

// Pid settings of the container
// It is for PID namespace setting. Usually different containers
// have their own PID namespace, however this specifies to use
// an existing PID namespace.
// Joining the host's PID namespace is currently the only supported
// option.
type Pid struct {
	HostPid bool `json:"host_pid"`
}

// UTS settings of the container
// It is for UTS namespace setting. Usually different containers
// have their own UTS namespace, however this specifies to use
// an existing UTS namespace.
// Joining the host's UTS namespace is currently the only supported
// option.
type UTS struct {
	HostUTS bool `json:"host_uts"`
}

// Resources contains all resource configs for a driver.
// Currently these are all for cgroup configs.
// TODO Windows: Factor out ulimit.Rlimit
type Resources struct {
	Memory            int64            `json:"memory"`
	MemorySwap        int64            `json:"memory_swap"`
	MemoryReservation int64            `json:"memory_reservation"`
	KernelMemory      int64            `json:"kernel_memory"`
	CPUShares         int64            `json:"cpu_shares"`
	CpusetCpus        string           `json:"cpuset_cpus"`
	CpusetMems        string           `json:"cpuset_mems"`
	CPUPeriod         int64            `json:"cpu_period"`
	CPUQuota          int64            `json:"cpu_quota"`
	BlkioWeight       uint16           `json:"blkio_weight"`
	Rlimits           []*ulimit.Rlimit `json:"rlimits"`
	OomKillDisable    bool             `json:"oom_kill_disable"`
	MemorySwappiness  int64            `json:"memory_swappiness"`
}

// ResourceStats contains information about resource usage by a container.
type ResourceStats struct {
	*libcontainer.Stats
	Read        time.Time `json:"read"`
	MemoryLimit int64     `json:"memory_limit"`
	SystemUsage uint64    `json:"system_usage"`
}

// Mount contains information for a mount operation.
type Mount struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Writable    bool   `json:"writable"`
	Private     bool   `json:"private"`
	Slave       bool   `json:"slave"`
}

// User contains the uid and gid representing a Unix user
type User struct {
	UID int `json:"root_uid"`
	GID int `json:"root_gid"`
}

// ProcessConfig describes a process that will be run inside a container.
type ProcessConfig struct {
	exec.Cmd `json:"-"`

	Privileged  bool     `json:"privileged"`
	User        string   `json:"user"`
	Tty         bool     `json:"tty"`
	Entrypoint  string   `json:"entrypoint"`
	Arguments   []string `json:"arguments"`
	Terminal    Terminal `json:"-"` // standard or tty terminal
	Console     string   `json:"-"` // dev/console path
	ConsoleSize [2]int   `json:"-"` // h,w of initial console size
}

// Command wraps an os/exec.Cmd to add more metadata
//
// TODO Windows: Factor out unused fields such as LxcConfig, AppArmorProfile,
// and CgroupParent.
type Command struct {
	ID                 string            `json:"id"`
	Rootfs             string            `json:"rootfs"` // root fs of the container
	ReadonlyRootfs     bool              `json:"readonly_rootfs"`
	InitPath           string            `json:"initpath"` // dockerinit
	WorkingDir         string            `json:"working_dir"`
	ConfigPath         string            `json:"config_path"` // this should be able to be removed when the lxc template is moved into the driver
	Network            *Network          `json:"network"`
	Ipc                *Ipc              `json:"ipc"`
	Pid                *Pid              `json:"pid"`
	UTS                *UTS              `json:"uts"`
	RemappedRoot       *User             `json:"remap_root"`
	UIDMapping         []idtools.IDMap   `json:"uidmapping"`
	GIDMapping         []idtools.IDMap   `json:"gidmapping"`
	Resources          *Resources        `json:"resources"`
	Mounts             []Mount           `json:"mounts"`
	AllowedDevices     []*configs.Device `json:"allowed_devices"`
	AutoCreatedDevices []*configs.Device `json:"autocreated_devices"`
	CapAdd             []string          `json:"cap_add"`
	CapDrop            []string          `json:"cap_drop"`
	GroupAdd           []string          `json:"group_add"`
	ContainerPid       int               `json:"container_pid"`  // the pid for the process inside a container
	ProcessConfig      ProcessConfig     `json:"process_config"` // Describes the init process of the container.
	ProcessLabel       string            `json:"process_label"`
	MountLabel         string            `json:"mount_label"`
	LxcConfig          []string          `json:"lxc_config"`
	AppArmorProfile    string            `json:"apparmor_profile"`
	CgroupParent       string            `json:"cgroup_parent"` // The parent cgroup for this command.
	FirstStart         bool              `json:"first_start"`
	LayerPaths         []string          `json:"layer_paths"` // Windows needs to know the layer paths and folder for a command
	LayerFolder        string            `json:"layer_folder"`
	Hostname           string            `json:"hostname"` // Windows sets the hostname in the execdriver
}

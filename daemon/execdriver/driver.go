package execdriver

import (
	"errors"
	"io"
	"os/exec"
	"time"

	"github.com/opencontainers/runc/libcontainer"
)

// Context is a generic key value pair that allows
// arbitrary data to be sent
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

// CommonResources contains the resource configs for a driver that are
// common across platforms.
type CommonResources struct {
	Memory            int64  `json:"memory"`
	MemoryReservation int64  `json:"memory_reservation"`
	CPUShares         int64  `json:"cpu_shares"`
	BlkioWeight       uint16 `json:"blkio_weight"`
}

// ResourceStats contains information about resource usage by a container.
type ResourceStats struct {
	*libcontainer.Stats
	Read        time.Time `json:"read"`
	MemoryLimit int64     `json:"memory_limit"`
	SystemUsage uint64    `json:"system_usage"`
}

// CommonProcessConfig is the common platform agnostic part of the ProcessConfig
// structure that describes a process that will be run inside a container.
type CommonProcessConfig struct {
	exec.Cmd `json:"-"`

	Tty        bool     `json:"tty"`
	Entrypoint string   `json:"entrypoint"`
	Arguments  []string `json:"arguments"`
	Terminal   Terminal `json:"-"` // standard or tty terminal
}

// CommonCommand is the common platform agnostic part of the Command structure
// which wraps an os/exec.Cmd to add more metadata
type CommonCommand struct {
	ContainerPid  int           `json:"container_pid"` // the pid for the process inside a container
	ID            string        `json:"id"`
	InitPath      string        `json:"initpath"`    // dockerinit
	MountLabel    string        `json:"mount_label"` // TODO Windows. More involved, but can be factored out
	Mounts        []Mount       `json:"mounts"`
	Network       *Network      `json:"network"`
	ProcessConfig ProcessConfig `json:"process_config"` // Describes the init process of the container.
	ProcessLabel  string        `json:"process_label"`  // TODO Windows. More involved, but can be factored out
	Resources     *Resources    `json:"resources"`
	Rootfs        string        `json:"rootfs"` // root fs of the container
	WorkingDir    string        `json:"working_dir"`
}

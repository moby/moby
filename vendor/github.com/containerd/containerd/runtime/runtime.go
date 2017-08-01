package runtime

import (
	"errors"
	"time"

	"github.com/containerd/containerd/specs"
)

var (
	// ErrContainerExited is returned when access to an exited
	// container is attempted
	ErrContainerExited = errors.New("containerd: container has exited")
	// ErrProcessNotExited is returned when trying to retrieve the exit
	// status of an alive process
	ErrProcessNotExited = errors.New("containerd: process has not exited")
	// ErrContainerNotStarted is returned when a container fails to
	// start without error from the shim or the OCI runtime
	ErrContainerNotStarted = errors.New("containerd: container not started")
	// ErrContainerStartTimeout is returned if a container takes too
	// long to start
	ErrContainerStartTimeout = errors.New("containerd: container did not start before the specified timeout")
	// ErrShimExited is returned if the shim or the contianer's init process
	// exits before completing
	ErrShimExited = errors.New("containerd: shim exited before container process was started")

	errNoPidFile         = errors.New("containerd: no process pid file found")
	errInvalidPidInt     = errors.New("containerd: process pid is invalid")
	errContainerNotFound = errors.New("containerd: container not found")
	errNotImplemented    = errors.New("containerd: not implemented")
)

const (
	// ExitFile holds the name of the pipe used to monitor process
	// exit
	ExitFile = "exit"
	// ExitStatusFile holds the name of the file where the container
	// exit code is to be written
	ExitStatusFile = "exitStatus"
	// StateFile holds the name of the file where the container state
	// is written
	StateFile = "state.json"
	// ControlFile holds the name of the pipe used to control the shim
	ControlFile = "control"
	// InitProcessID holds the special ID used for the very first
	// container's process
	InitProcessID = "init"
	// StartTimeFile holds the name of the file in which the process
	// start time is saved
	StartTimeFile = "starttime"

	// UnknownStatus is the value returned when a process exit
	// status cannot be determined
	UnknownStatus = 255
)

// Checkpoint holds information regarding a container checkpoint
type Checkpoint struct {
	// Timestamp is the time that checkpoint happened
	Created time.Time `json:"created"`
	// Name is the name of the checkpoint
	Name string `json:"name"`
	// TCP checkpoints open tcp connections
	TCP bool `json:"tcp"`
	// UnixSockets persists unix sockets in the checkpoint
	UnixSockets bool `json:"unixSockets"`
	// Shell persists tty sessions in the checkpoint
	Shell bool `json:"shell"`
	// Exit exits the container after the checkpoint is finished
	Exit bool `json:"exit"`
	// EmptyNS tells CRIU to omit a specified namespace
	EmptyNS []string `json:"emptyNS,omitempty"`
}

// PlatformProcessState container platform-specific fields in the ProcessState structure
type PlatformProcessState struct {
	Checkpoint string `json:"checkpoint"`
	RootUID    int    `json:"rootUID"`
	RootGID    int    `json:"rootGID"`
}

// State represents a container state
type State string

// Resource regroups the various container limits that can be updated
type Resource struct {
	CPUShares          int64
	BlkioWeight        uint16
	CPUPeriod          int64
	CPUQuota           int64
	CpusetCpus         string
	CpusetMems         string
	KernelMemory       int64
	KernelTCPMemory    int64
	Memory             int64
	MemoryReservation  int64
	MemorySwap         int64
	PidsLimit          int64
	CPURealtimePeriod  uint64
	CPURealtimdRuntime int64
}

// Possible container states
const (
	Paused  = State("paused")
	Stopped = State("stopped")
	Running = State("running")
)

type state struct {
	Bundle      string   `json:"bundle"`
	Labels      []string `json:"labels"`
	Stdin       string   `json:"stdin"`
	Stdout      string   `json:"stdout"`
	Stderr      string   `json:"stderr"`
	Runtime     string   `json:"runtime"`
	RuntimeArgs []string `json:"runtimeArgs"`
	Shim        string   `json:"shim"`
	NoPivotRoot bool     `json:"noPivotRoot"`
}

// ProcessState holds the process OCI specs along with various fields
// required by containerd
type ProcessState struct {
	specs.ProcessSpec
	Exec        bool     `json:"exec"`
	Stdin       string   `json:"containerdStdin"`
	Stdout      string   `json:"containerdStdout"`
	Stderr      string   `json:"containerdStderr"`
	RuntimeArgs []string `json:"runtimeArgs"`
	NoPivotRoot bool     `json:"noPivotRoot"`

	PlatformProcessState
}

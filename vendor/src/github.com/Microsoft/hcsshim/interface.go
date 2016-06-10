package hcsshim

import (
	"errors"
	"io"
	"time"
)

var (
	// ErrInvalidNotificationType is an error encountered when an invalid notification type is used
	ErrInvalidNotificationType = errors.New("hcsshim: invalid notification type")

	// ErrTimeout is an error encountered when waiting on a notification times out
	ErrTimeout = errors.New("hcsshim: timeout waiting for notification")

	// ErrHandleClose is an error returned when the handle generating the notification being waited on has been closed
	ErrHandleClose = errors.New("hcsshim: the handle generating this notification has been closed")

	// ErrInvalidProcessState is an error encountered when the process is not in a valid state for the requested operation
	ErrInvalidProcessState = errors.New("the process is in an invalid state for the attempted operation")

	// ErrUnexpectedContainerExit is the error returned when a container exits while waiting for
	// a different expected notification
	ErrUnexpectedContainerExit = errors.New("unexpected container exit")

	// ErrUnexpectedProcessAbort is the error returned when communication with the compute service
	// is lost while waiting for a notification
	ErrUnexpectedProcessAbort = errors.New("lost communication with compute service")
)

// ProcessConfig is used as both the input of Container.CreateProcess
// and to convert the parameters to JSON for passing onto the HCS
type ProcessConfig struct {
	ApplicationName  string
	CommandLine      string
	WorkingDirectory string
	Environment      map[string]string
	EmulateConsole   bool
	CreateStdInPipe  bool
	CreateStdOutPipe bool
	CreateStdErrPipe bool
	ConsoleSize      [2]int
}

type Layer struct {
	ID   string
	Path string
}

type MappedDir struct {
	HostPath      string
	ContainerPath string
	ReadOnly      bool
}

type HvRuntime struct {
	ImagePath string `json:",omitempty"`
}

// ContainerConfig is used as both the input of CreateContainer
// and to convert the parameters to JSON for passing onto the HCS
// TODO Windows: @darrenstahlmsft Add ProcessorCount
type ContainerConfig struct {
	SystemType              string      // HCS requires this to be hard-coded to "Container"
	Name                    string      // Name of the container. We use the docker ID.
	Owner                   string      // The management platform that created this container
	IsDummy                 bool        // Used for development purposes.
	VolumePath              string      // Windows volume path for scratch space
	IgnoreFlushesDuringBoot bool        // Optimization hint for container startup in Windows
	LayerFolderPath         string      // Where the layer folders are located
	Layers                  []Layer     // List of storage layers
	Credentials             string      `json:",omitempty"` // Credentials information
	ProcessorWeight         uint64      `json:",omitempty"` // CPU Shares 0..10000 on Windows; where 0 will be omitted and HCS will default.
	ProcessorMaximum        int64       `json:",omitempty"` // CPU maximum usage percent 1..100
	StorageIOPSMaximum      uint64      `json:",omitempty"` // Maximum Storage IOPS
	StorageBandwidthMaximum uint64      `json:",omitempty"` // Maximum Storage Bandwidth in bytes per second
	StorageSandboxSize      uint64      `json:",omitempty"` // Size in bytes that the container system drive should be expanded to if smaller
	MemoryMaximumInMB       int64       `json:",omitempty"` // Maximum memory available to the container in Megabytes
	HostName                string      // Hostname
	MappedDirectories       []MappedDir // List of mapped directories (volumes/mounts)
	SandboxPath             string      // Location of unmounted sandbox (used for Hyper-V containers)
	HvPartition             bool        // True if it a Hyper-V Container
	EndpointList            []string    // List of networking endpoints to be attached to container
	HvRuntime               *HvRuntime  // Hyper-V container settings
	Servicing               bool        // True if this container is for servicing
}

const (
	notificationTypeNone           string = "None"
	notificationTypeGracefulExit   string = "GracefulExit"
	notificationTypeForcedExit     string = "ForcedExit"
	notificationTypeUnexpectedExit string = "UnexpectedExit"
	notificationTypeReboot         string = "Reboot"
	notificationTypeConstructed    string = "Constructed"
	notificationTypeStarted        string = "Started"
	notificationTypePaused         string = "Paused"
	notificationTypeUnknown        string = "Unknown"
)

// Container represents a created (but not necessarily running) container.
type Container interface {
	// Start synchronously starts the container.
	Start() error

	// Shutdown requests a container shutdown, but it may not actually be shutdown until Wait() succeeds.
	Shutdown() error

	// Terminate requests a container terminate, but it may not actually be terminated until Wait() succeeds.
	Terminate() error

	// Waits synchronously waits for the container to shutdown or terminate.
	Wait() error

	// WaitTimeout synchronously waits for the container to terminate or the duration to elapse. It
	// returns false if timeout occurs.
	WaitTimeout(time.Duration) error

	// Pause pauses the execution of a container.
	Pause() error

	// Resume resumes the execution of a container.
	Resume() error

	// HasPendingUpdates returns true if the container has updates pending to install.
	HasPendingUpdates() (bool, error)

	// CreateProcess launches a new process within the container.
	CreateProcess(c *ProcessConfig) (Process, error)

	// OpenProcess gets an interface to an existing process within the container.
	OpenProcess(pid int) (Process, error)

	// Close cleans up any state associated with the container but does not terminate or wait for it.
	Close() error
}

// Process represents a running or exited process.
type Process interface {
	// Pid returns the process ID of the process within the container.
	Pid() int

	// Kill signals the process to terminate but does not wait for it to finish terminating.
	Kill() error

	// Wait waits for the process to exit.
	Wait() error

	// WaitTimeout waits for the process to exit or the duration to elapse. It returns
	// false if timeout occurs.
	WaitTimeout(time.Duration) error

	// ExitCode returns the exit code of the process. The process must have
	// already terminated.
	ExitCode() (int, error)

	// ResizeConsole resizes the console of the process.
	ResizeConsole(width, height uint16) error

	// Stdio returns the stdin, stdout, and stderr pipes, respectively. Closing
	// these pipes does not close the underlying pipes; it should be possible to
	// call this multiple times to get multiple interfaces.
	Stdio() (io.WriteCloser, io.ReadCloser, io.ReadCloser, error)

	// CloseStdin closes the write side of the stdin pipe so that the process is
	// notified on the read side that there is no more data in stdin.
	CloseStdin() error

	// Close cleans up any state associated with the process but does not kill
	// or wait on it.
	Close() error
}

package hcsshim

import (
	"io"
	"time"
)

// ProcessConfig is used as both the input of Container.CreateProcess
// and to convert the parameters to JSON for passing onto the HCS
type ProcessConfig struct {
	ApplicationName  string
	CommandLine      string
	User             string
	WorkingDirectory string
	Environment      map[string]string
	EmulateConsole   bool
	CreateStdInPipe  bool
	CreateStdOutPipe bool
	CreateStdErrPipe bool
	ConsoleSize      [2]uint
}

type Layer struct {
	ID   string
	Path string
}

type MappedDir struct {
	HostPath         string
	ContainerPath    string
	ReadOnly         bool
	BandwidthMaximum uint64
	IOPSMaximum      uint64
}

type HvRuntime struct {
	ImagePath    string `json:",omitempty"`
	SkipTemplate bool   `json:",omitempty"`
}

// ContainerConfig is used as both the input of CreateContainer
// and to convert the parameters to JSON for passing onto the HCS
type ContainerConfig struct {
	SystemType                 string      // HCS requires this to be hard-coded to "Container"
	Name                       string      // Name of the container. We use the docker ID.
	Owner                      string      // The management platform that created this container
	IsDummy                    bool        // Used for development purposes.
	VolumePath                 string      `json:",omitempty"` // Windows volume path for scratch space. Used by Windows Server Containers only. Format \\?\\Volume{GUID}
	IgnoreFlushesDuringBoot    bool        // Optimization hint for container startup in Windows
	LayerFolderPath            string      `json:",omitempty"` // Where the layer folders are located. Used by Windows Server Containers only. Format  %root%\windowsfilter\containerID
	Layers                     []Layer     // List of storage layers. Required for Windows Server and Hyper-V Containers. Format ID=GUID;Path=%root%\windowsfilter\layerID
	Credentials                string      `json:",omitempty"` // Credentials information
	ProcessorCount             uint32      `json:",omitempty"` // Number of processors to assign to the container.
	ProcessorWeight            uint64      `json:",omitempty"` // CPU Shares 0..10000 on Windows; where 0 will be omitted and HCS will default.
	ProcessorMaximum           int64       `json:",omitempty"` // CPU maximum usage percent 1..100
	StorageIOPSMaximum         uint64      `json:",omitempty"` // Maximum Storage IOPS
	StorageBandwidthMaximum    uint64      `json:",omitempty"` // Maximum Storage Bandwidth in bytes per second
	StorageSandboxSize         uint64      `json:",omitempty"` // Size in bytes that the container system drive should be expanded to if smaller
	MemoryMaximumInMB          int64       `json:",omitempty"` // Maximum memory available to the container in Megabytes
	HostName                   string      // Hostname
	MappedDirectories          []MappedDir // List of mapped directories (volumes/mounts)
	SandboxPath                string      `json:",omitempty"` // Location of unmounted sandbox. Used by Hyper-V containers only. Format %root%\windowsfilter
	HvPartition                bool        // True if it a Hyper-V Container
	EndpointList               []string    // List of networking endpoints to be attached to container
	NetworkSharedContainerName string      `json:",omitempty"` // Name (ID) of the container that we will share the network stack with.
	HvRuntime                  *HvRuntime  `json:",omitempty"` // Hyper-V container settings. Used by Hyper-V containers only. Format ImagePath=%root%\BaseLayerID\UtilityVM
	Servicing                  bool        // True if this container is for servicing
	AllowUnqualifiedDNSQuery   bool        // True to allow unqualified DNS name resolution
	DNSSearchList              string      `json:",omitempty"` // Comma seperated list of DNS suffixes to use for name resolution
}

type ComputeSystemQuery struct {
	IDs    []string `json:"Ids,omitempty"`
	Types  []string `json:",omitempty"`
	Names  []string `json:",omitempty"`
	Owners []string `json:",omitempty"`
}

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

	// Statistics returns statistics for a container.
	Statistics() (Statistics, error)

	// ProcessList returns details for the processes in a container.
	ProcessList() ([]ProcessListItem, error)

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

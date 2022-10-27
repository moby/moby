package types // import "github.com/docker/docker/libcontainerd/types"

import (
	"context"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// EventType represents a possible event from libcontainerd
type EventType string

// Event constants used when reporting events
const (
	EventUnknown     EventType = "unknown"
	EventExit        EventType = "exit"
	EventOOM         EventType = "oom"
	EventCreate      EventType = "create"
	EventStart       EventType = "start"
	EventExecAdded   EventType = "exec-added"
	EventExecStarted EventType = "exec-started"
	EventPaused      EventType = "paused"
	EventResumed     EventType = "resumed"
)

// EventInfo contains the event info
type EventInfo struct {
	ContainerID string
	ProcessID   string
	Pid         uint32
	ExitCode    uint32
	ExitedAt    time.Time
	Error       error
}

// Backend defines callbacks that the client of the library needs to implement.
type Backend interface {
	ProcessEvent(containerID string, event EventType, ei EventInfo) error
}

// Process of a container
type Process interface {
	// Pid is the system specific process id
	Pid() uint32
	// Kill sends the provided signal to the process
	Kill(ctx context.Context, signal syscall.Signal) error
	// Resize changes the width and height of the process's terminal
	Resize(ctx context.Context, width, height uint32) error
	// Delete removes the process and any resources allocated returning the exit status
	Delete(context.Context) (*containerd.ExitStatus, error)
}

// Client provides access to containerd features.
type Client interface {
	Version(ctx context.Context) (containerd.Version, error)
	// LoadContainer loads the metadata for a container from containerd.
	LoadContainer(ctx context.Context, containerID string) (Container, error)
	// NewContainer creates a new containerd container.
	NewContainer(ctx context.Context, containerID string, spec *specs.Spec, shim string, runtimeOptions interface{}, opts ...containerd.NewContainerOpts) (Container, error)
}

// Container provides access to a containerd container.
type Container interface {
	Start(ctx context.Context, checkpointDir string, withStdin bool, attachStdio StdioCallback) (Task, error)
	Task(ctx context.Context) (Task, error)
	// AttachTask returns the current task for the container and reattaches
	// to the IO for the running task. If no task exists for the container
	// a NotFound error is returned.
	//
	// Clients must make sure that only one reader is attached to the task.
	AttachTask(ctx context.Context, attachStdio StdioCallback) (Task, error)
	// Delete removes the container and associated resources
	Delete(context.Context) error
}

// Task provides access to a running containerd container.
type Task interface {
	Process
	// Pause suspends the execution of the task
	Pause(context.Context) error
	// Resume the execution of the task
	Resume(context.Context) error
	Stats(ctx context.Context) (*Stats, error)
	// Pids returns a list of system specific process ids inside the task
	Pids(context.Context) ([]containerd.ProcessInfo, error)
	Summary(ctx context.Context) ([]Summary, error)
	// ForceDelete forcefully kills the task's processes and deletes the task
	ForceDelete(context.Context) error
	// Status returns the executing status of the task
	Status(ctx context.Context) (containerd.Status, error)
	// Exec creates and starts a new process inside the task
	Exec(ctx context.Context, processID string, spec *specs.Process, withStdin bool, attachStdio StdioCallback) (Process, error)
	UpdateResources(ctx context.Context, resources *Resources) error
	CreateCheckpoint(ctx context.Context, checkpointDir string, exit bool) error
}

// StdioCallback is called to connect a container or process stdio.
type StdioCallback func(io *cio.DirectIO) (cio.IO, error)

// InitProcessName is the name given to the first process of a container
const InitProcessName = "init"

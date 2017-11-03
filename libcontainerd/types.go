package libcontainerd

import (
	"context"
	"io"
	"time"

	"github.com/containerd/containerd"
	"github.com/opencontainers/runtime-spec/specs-go"
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

// Status represents the current status of a container
type Status string

// Possible container statuses
const (
	// Running indicates the process is currently executing
	StatusRunning Status = "running"
	// Created indicates the process has been created within containerd but the
	// user's defined process has not started
	StatusCreated Status = "created"
	// Stopped indicates that the process has ran and exited
	StatusStopped Status = "stopped"
	// Paused indicates that the process is currently paused
	StatusPaused Status = "paused"
	// Pausing indicates that the process is currently switching from a
	// running state into a paused state
	StatusPausing Status = "pausing"
	// Unknown indicates that we could not determine the status from the runtime
	StatusUnknown Status = "unknown"
)

// Remote on Linux defines the accesspoint to the containerd grpc API.
// Remote on Windows is largely an unimplemented interface as there is
// no remote containerd.
type Remote interface {
	// Client returns a new Client instance connected with given Backend.
	NewClient(namespace string, backend Backend) (Client, error)
	// Cleanup stops containerd if it was started by libcontainerd.
	// Note this is not used on Windows as there is no remote containerd.
	Cleanup()
}

// RemoteOption allows to configure parameters of remotes.
// This is unused on Windows.
type RemoteOption interface {
	Apply(Remote) error
}

// EventInfo contains the event info
type EventInfo struct {
	ContainerID string
	ProcessID   string
	Pid         uint32
	ExitCode    uint32
	ExitedAt    time.Time
	OOMKilled   bool
	// Windows Only field
	UpdatePending bool
}

// Backend defines callbacks that the client of the library needs to implement.
type Backend interface {
	ProcessEvent(containerID string, event EventType, ei EventInfo) error
}

// Client provides access to containerd features.
type Client interface {
	Version(ctx context.Context) (containerd.Version, error)

	Restore(ctx context.Context, containerID string, attachStdio StdioCallback) (alive bool, pid int, err error)

	Create(ctx context.Context, containerID string, spec *specs.Spec, runtimeOptions interface{}) error
	Start(ctx context.Context, containerID, checkpointDir string, withStdin bool, attachStdio StdioCallback) (pid int, err error)
	SignalProcess(ctx context.Context, containerID, processID string, signal int) error
	Exec(ctx context.Context, containerID, processID string, spec *specs.Process, withStdin bool, attachStdio StdioCallback) (int, error)
	ResizeTerminal(ctx context.Context, containerID, processID string, width, height int) error
	CloseStdin(ctx context.Context, containerID, processID string) error
	Pause(ctx context.Context, containerID string) error
	Resume(ctx context.Context, containerID string) error
	Stats(ctx context.Context, containerID string) (*Stats, error)
	ListPids(ctx context.Context, containerID string) ([]uint32, error)
	Summary(ctx context.Context, containerID string) ([]Summary, error)
	DeleteTask(ctx context.Context, containerID string) (uint32, time.Time, error)
	Delete(ctx context.Context, containerID string) error
	Status(ctx context.Context, containerID string) (Status, error)

	UpdateResources(ctx context.Context, containerID string, resources *Resources) error
	CreateCheckpoint(ctx context.Context, containerID, checkpointDir string, exit bool) error
}

// StdioCallback is called to connect a container or process stdio.
type StdioCallback func(*IOPipe) (containerd.IO, error)

// IOPipe contains the stdio streams.
type IOPipe struct {
	Stdin    io.WriteCloser
	Stdout   io.ReadCloser
	Stderr   io.ReadCloser
	Terminal bool // Whether stderr is connected on Windows

	cancel context.CancelFunc
	config containerd.IOConfig
}

// ServerVersion contains version information as retrieved from the
// server
type ServerVersion struct {
}

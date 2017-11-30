// +build !windows

package proc

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/containerd/console"
	"github.com/pkg/errors"
)

// RuncRoot is the path to the root runc state directory
const RuncRoot = "/run/containerd/runc"

// Stdio of a process
type Stdio struct {
	Stdin    string
	Stdout   string
	Stderr   string
	Terminal bool
}

// IsNull returns true if the stdio is not defined
func (s Stdio) IsNull() bool {
	return s.Stdin == "" && s.Stdout == "" && s.Stderr == ""
}

// Process on a linux system
type Process interface {
	State
	// ID returns the id for the process
	ID() string
	// Pid returns the pid for the process
	Pid() int
	// ExitStatus returns the exit status
	ExitStatus() int
	// ExitedAt is the time the process exited
	ExitedAt() time.Time
	// Stdin returns the process STDIN
	Stdin() io.Closer
	// Stdio returns io information for the container
	Stdio() Stdio
	// Status returns the process status
	Status(context.Context) (string, error)
	// Wait blocks until the process has exited
	Wait()
}

// State of a process
type State interface {
	// Resize resizes the process console
	Resize(ws console.WinSize) error
	// Start execution of the process
	Start(context.Context) error
	// Delete deletes the process and its resourcess
	Delete(context.Context) error
	// Kill kills the process
	Kill(context.Context, uint32, bool) error
	// SetExited sets the exit status for the process
	SetExited(status int)
}

func stateName(v interface{}) string {
	switch v.(type) {
	case *runningState, *execRunningState:
		return "running"
	case *createdState, *execCreatedState, *createdCheckpointState:
		return "created"
	case *pausedState:
		return "paused"
	case *deletedState:
		return "deleted"
	case *stoppedState:
		return "stopped"
	}
	panic(errors.Errorf("invalid state %v", v))
}

// Platform handles platform-specific behavior that may differs across
// platform implementations
type Platform interface {
	CopyConsole(ctx context.Context, console console.Console, stdin, stdout, stderr string,
		wg, cwg *sync.WaitGroup) (console.Console, error)
	ShutdownConsole(ctx context.Context, console console.Console) error
	Close() error
}

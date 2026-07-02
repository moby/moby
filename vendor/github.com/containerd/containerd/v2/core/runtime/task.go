/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package runtime

import (
	"context"
	"time"

	"github.com/containerd/containerd/v2/pkg/protobuf/types"
)

// TaskInfo provides task specific information
type TaskInfo struct {
	ID        string
	Runtime   string
	Spec      []byte
	Namespace string
}

// Process is a runtime object for an executing process inside a container
type Process interface {
	// ID of the process
	ID() string
	// State returns the process state
	State(ctx context.Context) (State, error)
	// Kill signals a container
	Kill(ctx context.Context, signal uint32, all bool) error
	// ResizePty resizes the processes pty/console
	ResizePty(ctx context.Context, size ConsoleSize) error
	// CloseIO closes the processes IO
	CloseIO(ctx context.Context) error
	// Start the container's user defined process
	Start(ctx context.Context) error
	// Wait for the process to exit
	Wait(ctx context.Context) (*Exit, error)
}

// ExecProcess is a process spawned in container via Task.Exec call.
// The only difference from a regular `Process` is that exec process can delete self,
// while task process requires slightly more complex logic and needs to be deleted through the task manager.
type ExecProcess interface {
	Process

	// Delete deletes the process
	Delete(ctx context.Context) (*Exit, error)
}

// Task is the runtime object for an executing container
type Task interface {
	Process

	// PID of the process
	PID(ctx context.Context) (uint32, error)
	// Namespace that the task exists in
	Namespace() string
	// Pause pauses the container process
	Pause(ctx context.Context) error
	// Resume unpauses the container process
	Resume(ctx context.Context) error
	// Exec adds a process into the container
	Exec(ctx context.Context, id string, opts ExecOpts) (ExecProcess, error)
	// Pids returns all pids
	Pids(ctx context.Context) ([]ProcessInfo, error)
	// Checkpoint checkpoints a container to an image with live system data
	Checkpoint(ctx context.Context, path string, opts *types.Any) error
	// Update sets the provided resources to a running task
	Update(ctx context.Context, resources *types.Any, annotations map[string]string) error
	// Process returns a process within the task for the provided id
	Process(ctx context.Context, id string) (ExecProcess, error)
	// Stats returns runtime specific metrics for a task
	Stats(ctx context.Context) (*types.Any, error)
}

// ExecOpts provides additional options for additional processes running in a task
type ExecOpts struct {
	Spec *types.Any
	IO   IO
}

// ConsoleSize of a pty or windows terminal
type ConsoleSize struct {
	Width  uint32
	Height uint32
}

// Status is the runtime status of a task and/or process
type Status int

const (
	// CreatedStatus when a process has been created
	CreatedStatus Status = iota + 1
	// RunningStatus when a process is running
	RunningStatus
	// StoppedStatus when a process has stopped
	StoppedStatus
	// DeletedStatus when a process has been deleted
	DeletedStatus
	// PausedStatus when a process is paused
	PausedStatus
	// PausingStatus when a process is currently pausing
	PausingStatus
)

// State information for a process
type State struct {
	// Status is the current status of the container
	Status Status
	// Pid is the main process id for the container
	Pid uint32
	// ExitStatus of the process
	// Only valid if the Status is Stopped
	ExitStatus uint32
	// ExitedAt is the time at which the process exited
	// Only valid if the Status is Stopped
	ExitedAt time.Time
	Stdin    string
	Stdout   string
	Stderr   string
	Terminal bool
}

// ProcessInfo holds platform specific process information
type ProcessInfo struct {
	// Pid is the process ID
	Pid uint32
	// Info includes additional process information
	// Info varies by platform
	Info interface{}
}

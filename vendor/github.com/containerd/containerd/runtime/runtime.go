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

	"github.com/containerd/containerd/mount"
	"github.com/gogo/protobuf/types"
)

// IO holds process IO information
type IO struct {
	Stdin    string
	Stdout   string
	Stderr   string
	Terminal bool
}

// CreateOpts contains task creation data
type CreateOpts struct {
	// Spec is the OCI runtime spec
	Spec *types.Any
	// Rootfs mounts to perform to gain access to the container's filesystem
	Rootfs []mount.Mount
	// IO for the container's main process
	IO IO
	// Checkpoint digest to restore container state
	Checkpoint string
	// Options for the runtime and container
	Options *types.Any
}

// Exit information for a process
type Exit struct {
	Pid       uint32
	Status    uint32
	Timestamp time.Time
}

// PlatformRuntime is responsible for the creation and management of
// tasks and processes for a platform.
type PlatformRuntime interface {
	// ID of the runtime
	ID() string
	// Create creates a task with the provided id and options.
	Create(ctx context.Context, id string, opts CreateOpts) (Task, error)
	// Get returns a task.
	Get(context.Context, string) (Task, error)
	// Tasks returns all the current tasks for the runtime.
	// Any container runs at most one task at a time.
	Tasks(context.Context) ([]Task, error)
	// Delete removes the task in the runtime.
	Delete(context.Context, Task) (*Exit, error)
}

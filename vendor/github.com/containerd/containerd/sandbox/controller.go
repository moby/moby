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

package sandbox

import (
	"context"
	"fmt"
	"time"

	"github.com/containerd/containerd/api/types"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/typeurl/v2"
)

type CreateOptions struct {
	Rootfs []*types.Mount
	// Options are used to pass arbitrary options to the shim when creating a new sandbox.
	// CRI will use this to pass PodSandboxConfig.
	// Don't confuse this with Runtime options, which are passed at shim instance start
	// to setup global shim configuration.
	Options   typeurl.Any
	NetNSPath string
}

type CreateOpt func(*CreateOptions) error

// WithRootFS is used to create a sandbox with the provided rootfs mount
// TODO: Switch to mount.Mount once target added
func WithRootFS(m []*types.Mount) CreateOpt {
	return func(co *CreateOptions) error {
		co.Rootfs = m
		return nil
	}
}

// WithOptions allows passing arbitrary options when creating a new sandbox.
func WithOptions(options any) CreateOpt {
	return func(co *CreateOptions) error {
		var err error
		co.Options, err = typeurl.MarshalAny(options)
		if err != nil {
			return fmt.Errorf("failed to marshal sandbox options: %w", err)
		}

		return nil
	}
}

// WithNetNSPath used to assign network namespace path of a sandbox.
func WithNetNSPath(netNSPath string) CreateOpt {
	return func(co *CreateOptions) error {
		co.NetNSPath = netNSPath
		return nil
	}
}

type StopOptions struct {
	Timeout *time.Duration
}

type StopOpt func(*StopOptions)

func WithTimeout(timeout time.Duration) StopOpt {
	return func(so *StopOptions) {
		so.Timeout = &timeout
	}
}

// Controller is an interface to manage sandboxes at runtime.
// When running in sandbox mode, shim expected to implement `SandboxService`.
// Shim lifetimes are now managed manually via sandbox API by the containerd's client.
type Controller interface {
	// Create is used to initialize sandbox environment. (mounts, any)
	Create(ctx context.Context, sandboxID string, opts ...CreateOpt) error
	// Start will start previously created sandbox.
	Start(ctx context.Context, sandboxID string) (ControllerInstance, error)
	// Platform returns target sandbox OS that will be used by Controller.
	// containerd will rely on this to generate proper OCI spec.
	Platform(_ctx context.Context, _sandboxID string) (platforms.Platform, error)
	// Stop will stop sandbox instance
	Stop(ctx context.Context, sandboxID string, opts ...StopOpt) error
	// Wait blocks until sandbox process exits.
	Wait(ctx context.Context, sandboxID string) (ExitStatus, error)
	// Status will query sandbox process status. It is heavier than Ping call and must be used whenever you need to
	// gather metadata about current sandbox state (status, uptime, resource use, etc).
	Status(ctx context.Context, sandboxID string, verbose bool) (ControllerStatus, error)
	// Shutdown deletes and cleans all tasks and sandbox instance.
	Shutdown(ctx context.Context, sandboxID string) error
}

type ControllerInstance struct {
	SandboxID string
	Pid       uint32
	CreatedAt time.Time
	Labels    map[string]string
}

type ExitStatus struct {
	ExitStatus uint32
	ExitedAt   time.Time
}

type ControllerStatus struct {
	SandboxID string
	Pid       uint32
	State     string
	Info      map[string]string
	CreatedAt time.Time
	ExitedAt  time.Time
	Extra     typeurl.Any
}

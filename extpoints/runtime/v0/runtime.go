// Package runtimev0 defines the Moby-specific point through which the daemon
// offers container operations to extensions.
//
// The surface is deliberately minimal: it is the slice of the container
// backend that the builtin jobs extension drives runs with, not a general
// container API. Methods are added as extensions demonstrate a need for
// them.
//
// It is resolved locally because its contract carries Go semantics — a
// channel-based wait — that cannot cross a gRPC boundary. A transport-ready
// revision can succeed it once the framework generates streaming wire code.
package runtimev0

import (
	"context"

	"github.com/moby/extensions"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/daemon/server/backend"
)

// Point is the container runtime point. The daemon is its single provider.
var Point = extensions.DefineSinglePoint[Runtime]("org.mobyproject.extension.runtime.v0")

// Runtime is the container runtime surface the daemon provides. The
// container methods match *daemon.Daemon signatures method for method.
//
// Container operations only work once the daemon has restored its
// containers, which happens after extensions initialize: callers must wait
// for Ready before issuing container calls.
type Runtime interface {
	// Ready blocks until the provider can serve container operations, or
	// until ctx is done. It is the startup gate: the daemon builds the
	// extension host before its container backend is usable.
	Ready(ctx context.Context) error

	ContainerCreate(ctx context.Context, config backend.ContainerCreateConfig) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, name string, checkpoint string, checkpointDir string) error
	ContainerStop(ctx context.Context, name string, options backend.ContainerStopOptions) error
	ContainerRm(name string, config *backend.ContainerRmConfig) error
	// ContainerWait with WaitConditionNotRunning resolves on the container's
	// final exit, after its restart policy is exhausted. The returned channel
	// delivers exactly one status; it must never be closed without
	// delivering it.
	ContainerWait(ctx context.Context, name string, condition container.WaitCondition) (<-chan StateStatus, error)
}

// StateStatus reports a container's final exit. It mirrors the daemon
// container package's StateStatus so the daemon satisfies Runtime without an
// adapter, while keeping fakes trivial to build in tests.
type StateStatus interface {
	ExitCode() int
	Err() error
}

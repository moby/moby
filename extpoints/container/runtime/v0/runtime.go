//go:generate go tool mobyextgen

// Package containerruntimev0 defines the experimental point through which the
// daemon runs containers for extensions.
//
// The surface is deliberately minimal: it covers running containers, not a
// general container API.
// Methods and fields are added as extensions demonstrate a need for them.
package containerruntimev0

import (
	"context"

	"github.com/moby/extensions"
)

// ContainerRuntime creates and runs containers.
//
// Container operations only work once the daemon has restored its
// containers, which happens after extensions initialize.
// The point does not expose a startup gate: sequencing extension work after
// daemon readiness is a host concern.
type ContainerRuntime interface {
	// TODO: Add Create, taking a container definition declared as point
	// messages instead of JSON-encoded API types, so the contract is typed and
	// versioned with the point rather than the Engine API.

	// CreateFromAPI creates a container without starting it, from configs
	// in the Engine API's format.
	CreateFromAPI(ctx context.Context, req *CreateFromAPIRequest) (*CreateFromAPIReply, error)
	// Start starts a created or stopped container.
	Start(ctx context.Context, req *StartRequest) error
	// Stop stops a container using its configured stop signal and timeout,
	// falling back to the daemon's defaults.
	Stop(ctx context.Context, req *StopRequest) error
	// Remove removes a stopped container.
	Remove(ctx context.Context, req *RemoveRequest) error
}

// Point is single-cardinality: the daemon is the only provider.
// The daemon registers its builtin provider for extension dependencies,
// including out-of-process extensions, but does not publish it on the public
// daemon socket.
var Point = extensions.DefineSinglePoint[ContainerRuntime]("org.mobyproject.extension.container.runtime.v0")

// CreateFromAPIRequest describes the container to create.
// The configs use the JSON encoding of the matching types in
// github.com/moby/moby/api/types/container and
// github.com/moby/moby/api/types/network, as in the body of the Engine API's
// container create endpoint.
type CreateFromAPIRequest struct {
	// Name is the container name; empty lets the daemon generate one.
	Name string `pb:"1"`
	// Config is a JSON-encoded container.Config. Required.
	Config []byte `pb:"2"`
	// HostConfig is a JSON-encoded container.HostConfig. Optional.
	HostConfig []byte `pb:"3"`
	// NetworkingConfig is a JSON-encoded network.NetworkingConfig. Optional.
	NetworkingConfig []byte `pb:"4"`
}

// CreateFromAPIReply describes the created container.
type CreateFromAPIReply struct {
	// ID is the ID of the created container.
	ID string `pb:"1"`
	// Warnings are non-fatal issues encountered while creating the container.
	Warnings []string `pb:"2"`
}

// StartRequest identifies the container to start.
type StartRequest struct {
	// Container is the container's name or ID.
	Container string `pb:"1"`
}

// StopRequest identifies the container to stop.
type StopRequest struct {
	// Container is the container's name or ID.
	Container string `pb:"1"`
}

// RemoveRequest identifies the container to remove.
type RemoveRequest struct {
	// Container is the container's name or ID.
	Container string `pb:"1"`
}

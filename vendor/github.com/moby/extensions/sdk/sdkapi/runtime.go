//go:generate go run github.com/moby/extensions/cmd/mobyextgen -service=moby.extension.runtime.v1.Extension

// Package sdkapi defines the runtime protocol served by every out-of-process
// extension. It is generated from this Go-first contract and is not an
// extension point itself.
//
// Regenerate with `go generate ./internal/extensions/sdk/sdkapi/`.
package sdkapi

import "context"

// Extension is served by every out-of-process extension.
//
// The generator command names the gRPC service fully qualified because, unlike
// a point, there is no id to take the proto package from. Both halves are wire
// identifiers: extensions built against them are separate binaries, possibly
// built from an older tree, so neither may change without a new version.
type Extension interface {
	// Describe returns the extension's declaration.
	Describe(ctx context.Context, req *DescribeRequest) (*DescribeResponse, error)
	// Initialize runs the extension's Init. The daemon calls it after connecting
	// every extension, in dependency order, so that by the time an extension
	// initializes its dependencies are already initialized and reachable over the
	// callback channel. An Init error is returned as the RPC status.
	Initialize(ctx context.Context, req *InitializeRequest) (*InitializeResponse, error)
}

// DescribeRequest asks an extension to describe itself.
type DescribeRequest struct{}

// DescribeResponse carries the extension's declaration.
type DescribeResponse struct {
	Declaration *Declaration `pb:"1"`
}

// InitializeRequest asks an extension to run its Init.
type InitializeRequest struct{}

// InitializeResponse is the empty reply to a successful Initialize.
type InitializeResponse struct{}

// Declaration mirrors the in-process extension declaration over the wire.
type Declaration struct {
	ID           string             `pb:"1"`
	Providers    []PointDeclaration `pb:"2"`
	Dependencies []Dependency       `pb:"3"`
	Conflicts    []string           `pb:"4"`
	// OfferedPoints lists implemented Points eligible for Host-controlled
	// publication.
	OfferedPoints []string `pb:"5"`
	// ProviderServices records the fully-qualified gRPC service names registered
	// while serving each provider point. All of these services are available on
	// the per-extension socket for the daemon to call; only the host decides which
	// point's services are also published on the daemon API socket.
	ProviderServices []ProviderServices `pb:"6"`
}

// PointDeclaration names one point an extension provides.
type PointDeclaration struct {
	ID string `pb:"1"`
}

// ProviderServices are the gRPC services registered while serving one ordinary
// Point.
type ProviderServices struct {
	Point    string   `pb:"1"`
	Services []string `pb:"2"`
}

// Dependency is one point an extension depends on, optionally narrowed to the
// extension that must provide it.
type Dependency struct {
	Point     string `pb:"1"`
	Extension string `pb:"2"`
	Optional  bool   `pb:"3"`
}

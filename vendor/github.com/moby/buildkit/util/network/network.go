package network

import (
	"context"
	"io"
	"net"

	resourcestypes "github.com/moby/buildkit/executor/resources/types"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// Provider interface for Network
type Provider interface {
	io.Closer
	New(ctx context.Context, hostname string, opt NamespaceOptions) (Namespace, error)
}

type NamespaceOptions struct{}

// Namespace of network for workers
type Namespace interface {
	io.Closer
	// Set the namespace on the spec
	Set(*specs.Spec) error

	Sample() (*resourcestypes.NetworkSample, error)
}

type Dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

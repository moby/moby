package network

import (
	"context"
	"io"

	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// Provider interface for Network
type Provider interface {
	io.Closer
	New(ctx context.Context, hostname string) (Namespace, error)
}

// Namespace of network for workers
type Namespace interface {
	io.Closer
	// Set the namespace on the spec
	Set(*specs.Spec) error
}

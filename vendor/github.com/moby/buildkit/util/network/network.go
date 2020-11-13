package network

import (
	"io"

	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// Provider interface for Network
type Provider interface {
	New() (Namespace, error)
}

// Namespace of network for workers
type Namespace interface {
	io.Closer
	// Set the namespace on the spec
	Set(*specs.Spec) error
}

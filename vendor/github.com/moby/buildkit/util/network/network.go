package network

import (
	"io"

	"github.com/moby/buildkit/solver/pb"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// Default returns the default network provider set
func Default() map[pb.NetMode]Provider {
	return map[pb.NetMode]Provider{
		// FIXME: still uses host if no provider configured
		pb.NetMode_UNSET: NewHostProvider(),
		pb.NetMode_HOST:  NewHostProvider(),
		pb.NetMode_NONE:  NewNoneProvider(),
	}
}

// Provider interface for Network
type Provider interface {
	New() (Namespace, error)
}

// Namespace of network for workers
type Namespace interface {
	io.Closer
	// Set the namespace on the spec
	Set(*specs.Spec)
}

// NetworkOpts hold network options
type NetworkOpts struct {
	Type          string
	CNIConfigPath string
	CNIPluginPath string
}

package network

import (
	"context"
	"io"

	specs "github.com/opencontainers/runtime-spec/specs-go"
)

type Sample struct {
	RxBytes   int64 `json:"rxBytes,omitempty"`
	RxPackets int64 `json:"rxPackets,omitempty"`
	RxErrors  int64 `json:"rxErrors,omitempty"`
	RxDropped int64 `json:"rxDropped,omitempty"`
	TxBytes   int64 `json:"txBytes,omitempty"`
	TxPackets int64 `json:"txPackets,omitempty"`
	TxErrors  int64 `json:"txErrors,omitempty"`
	TxDropped int64 `json:"txDropped,omitempty"`
}

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

	Sample() (*Sample, error)
}

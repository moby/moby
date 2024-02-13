//go:build !windows
// +build !windows

package network

import (
	"context"

	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func NewHostProvider() Provider {
	return &host{}
}

type host struct {
}

func (h *host) New(_ context.Context, hostname string) (Namespace, error) {
	return &hostNS{}, nil
}

func (h *host) Close() error {
	return nil
}

type hostNS struct {
}

func (h *hostNS) Set(s *specs.Spec) error {
	return oci.WithHostNamespace(specs.NetworkNamespace)(nil, nil, nil, s)
}

func (h *hostNS) Close() error {
	return nil
}

func (h *hostNS) Sample() (*Sample, error) {
	return nil, nil
}

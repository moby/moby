//go:build !windows
// +build !windows

package network

import (
	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func NewHostProvider() Provider {
	return &host{}
}

type host struct {
}

func (h *host) New() (Namespace, error) {
	return &hostNS{}, nil
}

type hostNS struct {
}

func (h *hostNS) Set(s *specs.Spec) error {
	return oci.WithHostNamespace(specs.NetworkNamespace)(nil, nil, nil, s)
}

func (h *hostNS) Close() error {
	return nil
}

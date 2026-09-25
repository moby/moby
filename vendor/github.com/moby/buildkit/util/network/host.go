//go:build !windows

package network

import (
	"context"
	"net"

	"github.com/containerd/containerd/v2/pkg/oci"
	resourcestypes "github.com/moby/buildkit/executor/resources/types"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func NewHostProvider() Provider {
	return &host{}
}

type host struct {
}

func (h *host) New(_ context.Context, hostname string, _ NamespaceOptions) (Namespace, error) {
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

func (h *hostNS) Sample() (*resourcestypes.NetworkSample, error) {
	return nil, nil
}

func (h *hostNS) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, network, address)
}

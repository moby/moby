//go:build linux

package nftabler

import (
	"context"
	"net/netip"
)

func (n *network) AddEndpoint(ctx context.Context, epIPv4, epIPv6 netip.Addr) error {
	return nil
}

func (n *network) DelEndpoint(ctx context.Context, epIPv4, epIPv6 netip.Addr) error {
	return nil
}

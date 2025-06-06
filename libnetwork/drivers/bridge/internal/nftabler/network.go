//go:build linux

package nftabler

import (
	"context"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/drivers/bridge/internal/firewaller"
)

type network struct {
	config firewaller.NetworkConfig
	fw     *nftabler
}

func (nft *nftabler) NewNetwork(ctx context.Context, nc firewaller.NetworkConfig) (_ firewaller.Network, retErr error) {
	n := &network{
		fw:     nft,
		config: nc,
	}
	return n, nil
}

func (n *network) ReapplyNetworkLevelRules(ctx context.Context) error {
	log.G(ctx).Warn("ReapplyNetworkLevelRules is not implemented for nftables")
	return nil
}

func (n *network) DelNetworkLevelRules(ctx context.Context) error {
	return nil
}

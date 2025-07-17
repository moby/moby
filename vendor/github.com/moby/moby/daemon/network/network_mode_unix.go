//go:build !windows

package network

import (
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
)

const defaultNetwork = network.NetworkBridge

func isPreDefined(network string) bool {
	n := container.NetworkMode(network)
	return n.IsBridge() || n.IsHost() || n.IsNone() || n.IsDefault()
}

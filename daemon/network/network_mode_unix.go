//go:build !windows

package network

import (
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
)

const defaultNetwork = network.NetworkBridge

func isPreDefined(network string) bool {
	n := container.NetworkMode(network)
	return n.IsBridge() || n.IsHost() || n.IsNone() || n.IsDefault()
}

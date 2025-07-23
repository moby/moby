package network

import (
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
)

const defaultNetwork = network.NetworkNat

func isPreDefined(network string) bool {
	return !container.NetworkMode(network).IsUserDefined()
}

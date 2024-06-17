package network

import (
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
)

const defaultNetwork = network.NetworkNat

func isPreDefined(network string) bool {
	return !container.NetworkMode(network).IsUserDefined()
}

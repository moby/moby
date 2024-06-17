package network

import "github.com/docker/docker/api/types/container"

// DefaultNetwork is the name of the default network driver to use for containers
// on the daemon platform. The default for Linux containers is "bridge"
// ([network.NetworkBridge]), and "nat" ([network.NetworkNat]) for Windows
// containers.
const DefaultNetwork = defaultNetwork

// IsPredefined indicates if a network is predefined by the daemon.
func IsPredefined(network string) bool {
	return !container.NetworkMode(network).IsUserDefined()
}

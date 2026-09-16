package network

import "strings"

// DefaultNetwork is the name of the default network driver to use for containers
// on the daemon platform. The default for Linux containers is "bridge"
// ([network.NetworkBridge]), and "nat" ([network.NetworkNat]) for Windows
// containers.
const DefaultNetwork = defaultNetwork

// IsPredefined indicates if a network is predefined by the daemon.
func IsPredefined(network string) bool {
	// TODO(thaJeztah): check if we can align the check for both platforms
	return isPreDefined(network)
}

// IsReserved indicates if a network name is reserved and cannot be created by users.
// Reserved network names are "container" and any name starting with the
// "container:" prefix. Names matching this prefix are always parsed as
// container network-mode syntax (see [container.NetworkMode.IsContainer]),
// so a network with such a name could never be looked up or attached to by
// name through --network.
func IsReserved(networkName string) bool {
	return networkName == "container" || strings.HasPrefix(networkName, "container:")
}

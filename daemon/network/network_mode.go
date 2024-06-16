package network

// DefaultNetwork is the name of the default network driver to use for containers
// on the daemon platform. The default for Linux containers is "bridge"
// ([network.NetworkBridge]), and "nat" ([network.NetworkNat]) for Windows
// containers.
const DefaultNetwork = defaultNetwork

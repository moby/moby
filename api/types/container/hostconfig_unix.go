//go:build !windows

package container // import "github.com/docker/docker/api/types/container"

import "github.com/docker/docker/api/types/network"

// IsValid indicates if an isolation technology is valid
func (i Isolation) IsValid() bool {
	return i.IsDefault()
}

// NetworkName returns the name of the network stack.
func (n NetworkMode) NetworkName() string {
	if n.IsBridge() {
		return network.NetworkBridge
	} else if n.IsHost() {
		return network.NetworkHost
	} else if n.IsContainer() {
		return "container"
	} else if n.IsNone() {
		return network.NetworkNone
	} else if n.IsDefault() {
		return network.NetworkDefault
	} else if n.IsUserDefined() {
		return n.UserDefined()
	}
	return ""
}

// IsBridge indicates whether container uses the bridge network stack
func (n NetworkMode) IsBridge() bool {
	return n == network.NetworkBridge
}

// IsHost indicates whether container uses the host network stack.
func (n NetworkMode) IsHost() bool {
	return n == network.NetworkHost
}

// IsUserDefined indicates user-created network
func (n NetworkMode) IsUserDefined() bool {
	return !n.IsDefault() && !n.IsBridge() && !n.IsHost() && !n.IsNone() && !n.IsContainer()
}

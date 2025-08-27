package networkbackend

import "github.com/moby/moby/api/types/network"

// ConnectOptions represents the data to be used to connect a container to the
// network.
type ConnectOptions struct {
	Container      string
	EndpointConfig *network.EndpointSettings `json:",omitempty"`
}

// DisconnectOptions represents the data to be used to disconnect a container
// from the network.
type DisconnectOptions struct {
	Container string
	Force     bool
}

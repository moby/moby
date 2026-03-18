package networkbackend

import "github.com/moby/moby/api/types/network"

// ConnectRequest represents the data to be used to connect a container to the
// network.
type ConnectRequest struct {
	Container      string
	EndpointConfig *network.EndpointSettings `json:",omitempty"`
}

// DisconnectRequest represents the data to be used to disconnect a container
// from the network.
type DisconnectRequest struct {
	Container string
	Force     bool
}

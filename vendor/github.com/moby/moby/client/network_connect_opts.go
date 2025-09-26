package client

import "github.com/moby/moby/api/types/network"

// NetworkConnectOptions represents the data to be used to connect a container to the
// network.
type NetworkConnectOptions struct {
	Container      string
	EndpointConfig *network.EndpointSettings `json:",omitempty"`
}

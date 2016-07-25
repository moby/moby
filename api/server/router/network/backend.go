package network

import (
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/network"
)

// Backend is all the methods that need to be implemented
// to provide network specific functionality.
type Backend interface {
	FindNetwork(idName string) (*types.NetworkResource, error)
	GetNetworks() ([]types.NetworkResource, error)
	CreateNetwork(nc types.NetworkCreateRequest) (*types.NetworkCreateResponse, error)
	ConnectContainerToNetwork(containerID, networkID string, endpointConfig *network.EndpointSettings) error
	DisconnectContainerFromNetwork(containerID string, networkID string, force bool) error
	DeleteNetwork(name string) error
}

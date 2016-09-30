package network

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/libnetwork"
)

// Backend is all the methods that need to be implemented
// to provide network specific functionality.
type Backend interface {
	FindNetwork(idName string) (libnetwork.Network, error)
	GetNetworkByName(idName string) (libnetwork.Network, error)
	GetNetworksByID(partialID string) []libnetwork.Network
	GetNetworks() []libnetwork.Network
	CreateNetwork(nc types.NetworkCreateRequest) (*types.NetworkCreateResponse, error)
	ConnectContainerToNetwork(containerName, networkName string, endpointConfig *network.EndpointSettings) error
	DisconnectContainerFromNetwork(containerName string, networkName string, force bool) error
	DeleteNetwork(name string) error
}

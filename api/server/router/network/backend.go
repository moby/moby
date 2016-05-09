package network

import (
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/filters"
	"github.com/docker/engine-api/types/network"
	"github.com/docker/libnetwork"
)

// Backend is all the methods that need to be implemented
// to provide network specific functionality.
type Backend interface {
	FindNetwork(idName string) (libnetwork.Network, error)
	GetNetworkByName(idName string) (libnetwork.Network, error)
	GetNetworksByID(partialID string) []libnetwork.Network
	FilterNetworks(netFilters filters.Args) ([]libnetwork.Network, error)
	CreateNetwork(nc types.NetworkCreateRequest) (*types.NetworkCreateResponse, error)
	ConnectContainerToNetwork(containerName, networkName string, endpointConfig *network.EndpointSettings) error
	DisconnectContainerFromNetwork(containerName string, network libnetwork.Network, force bool) error
	DeleteNetwork(name string) error
}

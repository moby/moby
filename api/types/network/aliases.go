package network

import (
	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/api/types/network"
)

// CreateResponse NetworkCreateResponse
//
// OK response to NetworkCreate operation
type CreateResponse = network.CreateResponse

// EndpointSettings stores the network endpoint details
type EndpointSettings = network.EndpointSettings

// EndpointIPAMConfig represents IPAM configurations for the endpoint
type EndpointIPAMConfig = network.EndpointIPAMConfig

// NetworkSubnet describes a user-defined subnet for a specific network. It's only used to validate if an
// EndpointIPAMConfig is valid for a specific network.
type NetworkSubnet = network.NetworkSubnet

// IPAM represents IP Address Management
type IPAM = network.IPAM

// IPAMConfig represents IPAM configurations
type IPAMConfig = network.IPAMConfig

// ValidateIPAM checks whether the network's IPAM passed as argument is valid. It returns a joinError of the list of
// errors found.
func ValidateIPAM(ipam *network.IPAM, enableIPv6 bool) error {
	return network.ValidateIPAM(ipam, enableIPv6)
}

const (
	// NetworkDefault is a platform-independent alias to choose the platform-specific default network stack.
	NetworkDefault = network.NetworkDefault
	// NetworkHost is the name of the predefined network used when the NetworkMode host is selected (only available on Linux)
	NetworkHost = network.NetworkHost
	// NetworkNone is the name of the predefined network used when the NetworkMode none is selected (available on both Linux and Windows)
	NetworkNone = network.NetworkNone
	// NetworkBridge is the name of the default network on Linux
	NetworkBridge = network.NetworkBridge
	// NetworkNat is the name of the default network on Windows
	NetworkNat = network.NetworkNat
)

// CreateRequest is the request message sent to the server for network create call.
type CreateRequest = network.CreateRequest

// CreateOptions holds options to create a network.
type CreateOptions = network.CreateOptions

// ListOptions holds parameters to filter the list of networks with.
type ListOptions = network.ListOptions

// InspectOptions holds parameters to inspect network.
type InspectOptions = network.InspectOptions

// ConnectOptions represents the data to be used to connect a container to the
// network.
type ConnectOptions = network.ConnectOptions

// DisconnectOptions represents the data to be used to disconnect a container
// from the network.
type DisconnectOptions = network.DisconnectOptions

// Inspect is the body of the "get network" http response message.
type Inspect = network.Inspect

// Summary is used as response when listing networks. It currently is an alias
// for [Inspect], but may diverge in the future, as not all information may
// be included when listing networks.
type Summary = network.Summary

// Address represents an IP address
type Address = network.Address

// PeerInfo represents one peer of an overlay network
type PeerInfo = network.PeerInfo

// Task carries the information about one backend task
type Task = network.Task

// ServiceInfo represents service parameters with the list of service's tasks
type ServiceInfo = network.ServiceInfo

// EndpointResource contains network resources allocated and used for a
// container in a network.
type EndpointResource = network.EndpointResource

// NetworkingConfig represents the container's networking configuration for each of its interfaces
// Carries the networking configs specified in the `docker run` and `docker network connect` commands
type NetworkingConfig = network.NetworkingConfig

// ConfigReference specifies the source which provides a network's configuration
type ConfigReference = network.ConfigReference

// ValidateFilters validates the list of filter args with the available filters.
func ValidateFilters(filter filters.Args) error {
	return network.ValidateFilters(filter)
}

// PruneReport contains the response for Engine API:
// POST "/networks/prune"
type PruneReport = network.PruneReport

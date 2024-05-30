package network // import "github.com/docker/docker/api/types/network"

import (
	"github.com/docker/docker/api/types/filters"
)

const (
	// NetworkDefault is a platform-independent alias to choose the platform-specific default network stack.
	NetworkDefault = "default"
	// NetworkHost is the name of the predefined network used when the NetworkMode host is selected (only available on Linux)
	NetworkHost = "host"
	// NetworkNone is the name of the predefined network used when the NetworkMode none is selected (available on both Linux and Windows)
	NetworkNone = "none"
	// NetworkBridge is the name of the default network on Linux
	NetworkBridge = "bridge"
	// NetworkNat is the name of the default network on Windows
	NetworkNat = "nat"
)

// InspectOptions holds parameters to inspect network.
type InspectOptions struct {
	Scope   string
	Verbose bool
}

// ConnectOptions represents the data to be used to connect a container to the
// network.
type ConnectOptions struct {
	Container      string
	EndpointConfig *EndpointSettings `json:",omitempty"`
}

// DisconnectOptions represents the data to be used to disconnect a container
// from the network.
type DisconnectOptions struct {
	Container string
	Force     bool
}

// Address represents an IP address
type Address struct {
	Addr      string
	PrefixLen int
}

// PeerInfo represents one peer of an overlay network
type PeerInfo struct {
	Name string
	IP   string
}

// Task carries the information about one backend task
type Task struct {
	Name       string
	EndpointID string
	EndpointIP string
	Info       map[string]string
}

// ServiceInfo represents service parameters with the list of service's tasks
type ServiceInfo struct {
	VIP          string
	Ports        []string
	LocalLBIndex int
	Tasks        []Task
}

// EndpointResource contains network resources allocated and used for a
// container in a network.
type EndpointResource struct {
	Name        string
	EndpointID  string
	MacAddress  string
	IPv4Address string
	IPv6Address string
}

// NetworkingConfig represents the container's networking configuration for each of its interfaces
// Carries the networking configs specified in the `docker run` and `docker network connect` commands
type NetworkingConfig struct {
	EndpointsConfig map[string]*EndpointSettings // Endpoint configs for each connecting network
}

// ConfigReference specifies the source which provides a network's configuration
type ConfigReference struct {
	Network string
}

var acceptedFilters = map[string]bool{
	"dangling": true,
	"driver":   true,
	"id":       true,
	"label":    true,
	"name":     true,
	"scope":    true,
	"type":     true,
}

// ValidateFilters validates the list of filter args with the available filters.
func ValidateFilters(filter filters.Args) error {
	return filter.Validate(acceptedFilters)
}
